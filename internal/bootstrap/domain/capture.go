package domain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/aiengine"
	bootinfra "github.com/xxxsen/yamdc/internal/bootstrap/infra"
	bootrt "github.com/xxxsen/yamdc/internal/bootstrap/runtime"
	"github.com/xxxsen/yamdc/internal/browser"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/flarerr"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/translator"
)

// CaptureRuntimeConfig holds the parameters needed to build a CaptureRuntime,
// decoupled from the config package.
type CaptureRuntimeConfig struct {
	DataDir           string
	Proxy             string
	BrowserRemoteURL  string
	HTTPClient        bootinfra.HTTPClientConfig
	FlareSolverr      *bootinfra.FlareSolverrConfig
	Dependencies      []bootinfra.DependencySpec
	AIEngineName      string
	AIEngineArgs      any
	Translator        bootrt.TranslatorConfig
	EnablePigoFaceRec bool
}

// CaptureRuntime holds the runtime dependencies needed by a single capture run.
type CaptureRuntime struct {
	CLI        client.IHTTPClient
	CacheStore store.IStorage
	Engine     aiengine.IAIEngine
	Translator translator.ITranslator
	FaceRec    face.IFaceRec
	Nav        browser.INavigator
}

func (rt *CaptureRuntime) Close() {
	if rt.Nav != nil {
		_ = rt.Nav.Close()
	}
	if closer, ok := rt.CacheStore.(io.Closer); ok {
		_ = closer.Close()
	}
}

func BuildCaptureRuntime(ctx context.Context, cfg CaptureRuntimeConfig) (*CaptureRuntime, error) {
	cli, err := bootinfra.BuildHTTPClient(ctx, cfg.HTTPClient)
	if err != nil {
		return nil, fmt.Errorf("build http client: %w", err)
	}
	if cfg.FlareSolverr != nil {
		cli = flarerr.NewHTTPClient(cli, cfg.FlareSolverr.Host)
	}
	nav := browser.NewNavigator(&browser.Config{
		RemoteURL: cfg.BrowserRemoteURL,
		DataDir:   cfg.DataDir,
		Proxy:     cfg.Proxy,
	})
	cli = browser.NewHTTPClient(cli, nav)
	if err := bootinfra.InitDependencies(ctx, cli, cfg.DataDir, cfg.Dependencies); err != nil {
		return nil, fmt.Errorf("init dependencies: %w", err)
	}
	engine, err := bootrt.BuildAIEngine(ctx, cli, cfg.AIEngineName, cfg.AIEngineArgs)
	if err != nil && !errors.Is(err, bootrt.ErrAIEngineNotConfigured) {
		return nil, fmt.Errorf("build ai engine: %w", err)
	}
	cacheStore, err := bootinfra.BuildCacheStore(ctx, cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("build cache store: %w", err)
	}
	tr, err := bootrt.BuildTranslator(ctx, cfg.Translator, engine)
	if err != nil && !errors.Is(err, bootrt.ErrTranslatorNotConfigured) {
		logutil.GetLogger(ctx).Error("setup translator failed", zap.Error(err))
	}
	faceRec, err := bootrt.BuildFaceRecognizer(ctx, cfg.EnablePigoFaceRec, filepath.Join(cfg.DataDir, "models"))
	if err != nil {
		logutil.GetLogger(ctx).Error("init face recognizer failed", zap.Error(err))
	}
	return &CaptureRuntime{
		CLI: cli, CacheStore: cacheStore, Engine: engine,
		Translator: tr, FaceRec: faceRec, Nav: nav,
	}, nil
}

// CaptureConfig holds the parameters for BuildCapture, decoupled from config package.
type CaptureConfig struct {
	Naming                 string
	ScanDir                string
	SaveDir                string
	ExtraMediaExts         []string
	DiscardTranslatedTitle bool
	DiscardTranslatedPlot  bool
	EnableLinkMode         bool
}

func BuildCapture(
	cfg CaptureConfig,
	storage store.IStorage,
	sr searcher.ISearcher,
	ps []processor.IProcessor,
	movieIDCleaner movieidcleaner.Cleaner,
) (*capture.Capture, error) {
	opts := make([]capture.Option, 0, 10)
	opts = append(opts,
		capture.WithNamingRule(cfg.Naming),
		capture.WithScanDir(cfg.ScanDir),
		capture.WithSaveDir(cfg.SaveDir),
		capture.WithSeacher(sr),
		capture.WithProcessor(processor.NewGroup(ps)),
		capture.WithStorage(storage),
		capture.WithExtraMediaExtList(cfg.ExtraMediaExts),
		capture.WithMovieIDCleaner(movieIDCleaner),
		capture.WithTransalteTitleDiscard(cfg.DiscardTranslatedTitle),
		capture.WithTranslatedPlotDiscard(cfg.DiscardTranslatedPlot),
		capture.WithLinkMode(cfg.EnableLinkMode),
	)
	capt, err := capture.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create capture: %w", err)
	}
	return capt, nil
}
