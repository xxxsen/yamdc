package bootstrap

import (
	"context"
	"fmt"
	"io"

	"github.com/xxxsen/yamdc/internal/bootstrap/infra"
	"github.com/xxxsen/yamdc/internal/browser"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/flarerr"
)

// 基础设施层 (infra) 的启动 action 集合:
//   - 目录规范化 / 日志初始化 / 目录前置检查
//   - HTTP client、FlareSolverr / Browser 包装
//   - 外部依赖下载、Cache store 初始化
//
// 所有 action 函数签名与 `InitAction.Run` 保持一致。

func normalizeDirPathsAction(_ context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	if err := infra.NormalizeDirPaths(
		&c.DataDir, &c.ScanDir, &c.SaveDir, &c.LibraryDir,
	); err != nil {
		return fmt.Errorf("normalize dir paths: %w", err)
	}
	return nil
}

func initLoggerAction(_ context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	sc.Infra.Logger = infra.InitLogger(
		c.LogConfig.File, c.LogConfig.Level,
		int(c.LogConfig.FileCount), //nolint:gosec // bounded config value
		int(c.LogConfig.FileSize),  //nolint:gosec // bounded config value
		int(c.LogConfig.KeepDays),
		c.LogConfig.Console,
	)
	return nil
}

func precheckDirsServerAction(_ context.Context, sc *StartContext) error {
	if err := config.ValidateForServer(sc.Infra.Config); err != nil {
		return fmt.Errorf("server dir validation failed: %w", err)
	}
	return nil
}

func buildHTTPClientAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	cli, err := infra.BuildHTTPClient(ctx, toHTTPClientConfig(c))
	if err != nil {
		return fmt.Errorf("build http client: %w", err)
	}
	sc.Infra.HTTPClient = cli
	return nil
}

func buildBrowserClientAction(_ context.Context, sc *StartContext) error {
	if sc.Infra.Config.FlareSolverrConfig.Enable {
		sc.Infra.HTTPClient = flarerr.NewHTTPClient(
			sc.Infra.HTTPClient,
			sc.Infra.Config.FlareSolverrConfig.Host,
		)
	}
	nav := browser.NewNavigator(&browser.Config{
		RemoteURL: sc.Infra.Config.BrowserConfig.RemoteURL,
		DataDir:   sc.Infra.Config.DataDir,
		Proxy:     sc.Infra.Config.NetworkConfig.Proxy,
	})
	sc.Cleanup.Add("browser_navigator", func(context.Context) error {
		return nav.Close()
	})
	sc.Infra.HTTPClient = browser.NewHTTPClient(sc.Infra.HTTPClient, nav)
	return nil
}

func initDependenciesAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	if err := infra.InitDependencies(
		ctx, sc.Infra.HTTPClient, c.DataDir, toDependencySpecs(c.Dependencies),
	); err != nil {
		return fmt.Errorf("init dependencies: %w", err)
	}
	return nil
}

func buildCacheStoreAction(ctx context.Context, sc *StartContext) error {
	cacheStore, err := infra.BuildCacheStore(ctx, sc.Infra.Config.DataDir)
	if err != nil {
		return fmt.Errorf("build cache store: %w", err)
	}
	sc.Infra.CacheStore = cacheStore
	if closer, ok := cacheStore.(io.Closer); ok {
		sc.Cleanup.Add("cache_store", func(context.Context) error {
			return closer.Close()
		})
	}
	return nil
}
