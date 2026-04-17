package bootstrap

import (
	"context"
	"errors"
	"fmt"

	bootapp "github.com/xxxsen/yamdc/internal/bootstrap/app"
	"github.com/xxxsen/yamdc/internal/bootstrap/server"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/review"
	"github.com/xxxsen/yamdc/internal/scanner"
	plugineditor "github.com/xxxsen/yamdc/internal/searcher/plugin/editor"
	"github.com/xxxsen/yamdc/internal/web"
	"go.uber.org/zap"
)

// 应用层 (app) action 集合:
//   - 打开 app sqlite
//   - 组装 JobRepo / LogRepo / ScrapeRepo / ScanSvc / JobSvc / MediaSvc / Web API
//   - Job 恢复 / 启动 media 后台 goroutine
//   - 阻塞式 HTTP serve (收到 ctx cancel 后 graceful shutdown)

// errAppDBNotInitialized 由 healthz deep 探活返回, 表示 app DB 未初始化。
// 放在包级静态错误, 避免 err113 lint 告警。
var errAppDBNotInitialized = errors.New("app db not initialized")

func openAppDBAction(ctx context.Context, sc *StartContext) error {
	appDB, err := bootapp.OpenAppDB(ctx, sc.Infra.Config.DataDir)
	if err != nil {
		return fmt.Errorf("open app db: %w", err)
	}
	sc.App.AppDB = appDB
	sc.Cleanup.Add("app_db", func(context.Context) error {
		return appDB.Close()
	})
	return nil
}

func assembleServicesAction(_ context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	sc.App.JobRepo = repository.NewJobRepository(sc.App.AppDB.DB())
	sc.App.LogRepo = repository.NewLogRepository(sc.App.AppDB.DB())
	sc.App.ScrapeRepo = repository.NewScrapeDataRepository(sc.App.AppDB.DB())
	sc.App.ScanSvc = scanner.New(
		c.ScanDir, c.ExtraMediaExts,
		sc.App.JobRepo, sc.Domain.MovieIDCleaner,
	)
	sc.App.JobSvc = job.NewService(
		sc.App.JobRepo, sc.App.LogRepo, sc.App.ScrapeRepo,
		sc.Domain.Capture, sc.Infra.CacheStore,
	)
	// 3.2 reviewing 工作流独立: SaveReviewData / CropPosterFromCover / Import
	// 都挂在 review.Service 上, 它复用 job.Service 的 Claim/Finish/AddJobLog/
	// ResolveJobSourcePath/GetBlockingConflict 做协作, 依赖方向单向 (review → job)。
	sc.App.ReviewSvc = review.NewService(
		sc.App.JobSvc,
		sc.App.JobRepo, sc.App.ScrapeRepo,
		sc.Domain.Capture, sc.Infra.CacheStore,
	)
	sc.App.MediaSvc = medialib.NewService(
		sc.App.AppDB.DB(), c.LibraryDir, c.SaveDir,
	)
	sc.App.ReviewSvc.SetImportGuard(func(_ context.Context) error {
		if sc.App.MediaSvc.IsMoveRunning() {
			return ErrMoveToMediaLibRunning
		}
		return nil
	})
	// 按 LIFO 注册: 这两个 wait 会在 app_db/cache_store 关闭前执行,
	// 避免后台 goroutine 的 DB 写入撞上 sql.DB.Close() 导致的
	// "database is closed" 错误。
	sc.Cleanup.Add("wait_media_background", func(context.Context) error {
		sc.App.MediaSvc.WaitBackground()
		return nil
	})
	sc.Cleanup.Add("wait_job_worker", func(context.Context) error {
		sc.App.JobSvc.WaitQueuedJobs()
		return nil
	})
	editorSvc, err := plugineditor.NewService(sc.Infra.HTTPClient)
	if err != nil {
		return fmt.Errorf("init plugin editor service failed, err:%w", err)
	}
	sc.App.API = web.NewAPI(
		sc.App.JobRepo,
		sc.App.ScanSvc,
		sc.App.JobSvc,
		sc.App.ReviewSvc,
		c.SaveDir,
		sc.App.MediaSvc,
		sc.Infra.CacheStore,
		sc.Domain.MovieIDCleaner,
		sc.Domain.SearcherDebugger,
		sc.Domain.HandlerDebugger,
		editorSvc,
		buildHealthCheck(sc.App.AppDB),
	)
	return nil
}

// buildHealthCheck 构造 web 层深度健康检查函数。
// 若 appDB 尚未初始化 (例如单测里只启动到 open_app_db 之前),
// 返回 nil → /api/healthz?deep=1 会返回 "deep: skipped"。
func buildHealthCheck(appDB *repository.SQLite) web.HealthCheckFunc {
	if appDB == nil {
		return nil
	}
	return func(ctx context.Context) error {
		db := appDB.DB()
		if db == nil {
			return errAppDBNotInitialized
		}
		return db.PingContext(ctx)
	}
}

func recoverJobsAction(ctx context.Context, sc *StartContext) error {
	if err := sc.App.JobSvc.Recover(ctx); err != nil && sc.Infra.Logger != nil {
		sc.Infra.Logger.Error("recover processing jobs failed", zap.Error(err))
	}
	return nil
}

func startMediaServiceAction(ctx context.Context, sc *StartContext) error {
	sc.App.MediaSvc.Start(ctx)
	return nil
}

func serveHTTPAction(ctx context.Context, sc *StartContext) error {
	if err := server.ServeHTTP(
		ctx, sc.App.API, sc.Infra.Logger,
		sc.Infra.Config.ScanDir, sc.Infra.Config.DataDir,
	); err != nil {
		return fmt.Errorf("serve http: %w", err)
	}
	return nil
}
