package bootstrap

import (
	"context"
	"errors"
	"fmt"

	bootapp "github.com/xxxsen/yamdc/internal/bootstrap/app"
	"github.com/xxxsen/yamdc/internal/bootstrap/server"
	"github.com/xxxsen/yamdc/internal/cronscheduler"
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
	// CronScheduler 是进程级的定时任务编排器, 目前只挂 media library
	// 的 auto sync + log cleanup 两个 job。rootCtx 用 context.Background
	// 是故意的: cron job 的 "取消" 语义走 Scheduler.Stop (拒绝新 tick +
	// 等当前 job 返回), 不走 rootCtx cancel — 避免外层 ctx 被 cancel 时
	// 某个 job 正好执行到一半被硬拽断。job 内部的 long-running 流程 (例如
	// runFullSync) 仍然会走 MediaSvc.shutdownCtx, 由 stop_media_service
	// 单独 cancel, 是两套相互独立的生命周期。
	// 刻意不继承 startup ctx: cron 的 rootCtx 要贯穿整个进程生命周期,
	// 由 Scheduler.Stop 管理, 不能被 startup ctx 的 cancel 提前打挂。
	//nolint:contextcheck // see comment above
	sc.App.CronScheduler = cronscheduler.New(context.Background(), sc.Infra.Logger)
	registerAppCleanups(sc)
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

// registerAppCleanups 注册 app 层服务的 cleanup 条目。
//
// Cleanup 注册顺序决定了 LIFO 执行顺序 (后注册的先执行)。
// 本 scope 下期望的 LIFO 执行顺序:
//  1. stop_job_worker        (不再接新 scrape 任务)
//  2. stop_media_service     (cancel shutdownCtx, 正在跑的 runFullSync
//     / runMove 响应退出)
//  3. stop_cron_scheduler    (cron.Stop() 拒绝新 tick, 等运行中的 cron
//     job 收敛; 上一步已 cancel MediaSvc ctx,
//     所以 AutoSyncJob 里触发的 sync 能快速
//     退出, 不会让 cron 一直等)
//  4. wait_media_background  (阻塞等所有 bg goroutine 完成)
//  5. app_db / cache_store 关闭 (由 openAppDBAction 等更早注册的 cleanup 负责)
//
// 顺序反了会挂住:
//   - 先 stop_cron_scheduler 再 stop_media_service: cron 等 auto sync
//     job 完成, 但 auto sync 里的 runFullSync ctx 还没 cancel, 可能跑几分钟
//   - stop_media_service 早于 stop_job_worker: 正在跑的 scrape 任务可能
//     还会调到 media svc
//
// 因此注册时必须逆序写 (注册时 LIFO 尾部的条目是运行时的第一条)。
func registerAppCleanups(sc *StartContext) {
	sc.Cleanup.Add("wait_media_background", func(context.Context) error {
		sc.App.MediaSvc.WaitBackground()
		return nil
	})
	sc.Cleanup.Add("stop_cron_scheduler", func(context.Context) error {
		sc.App.CronScheduler.Stop()
		return nil
	})
	sc.Cleanup.Add("stop_media_service", func(context.Context) error {
		sc.App.MediaSvc.Stop()
		return nil
	})
	// 3.4: 用 Stop(ctx) 替代 WaitQueuedJobs(): 除了等排空, 还主动拒绝
	// cleanup 阶段 (HTTP 已 shutdown, 但 recover/其它 goroutine 理论上可能
	// 还在 Run) 的新入队请求, 避免 close(queue) 与 Run 的 channel send 竞争。
	sc.Cleanup.Add("stop_job_worker", func(ctx context.Context) error {
		return sc.App.JobSvc.Stop(ctx)
	})
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

// registerCronJobsAction 把进程级定时任务挂进 CronScheduler。
// 注册顺序也是 cron 触发时相同 tick 下的执行顺序 (robfig/cron 按注册顺序
// 依次触发): LogCleanupJob 先, AutoSyncJob 后 — 这样 03:00 的 tick 下
// 总是 "先裁旧日志、再看要不要同步", 日志表不会因为 sync 长耗时导致
// cleanup 被推迟。
//
// 新增定时任务时, 在这里追加一行 Register 即可, 不需要改其它地方。
func registerCronJobsAction(_ context.Context, sc *StartContext) error {
	if err := sc.App.CronScheduler.Register(medialib.NewLogCleanupJob(sc.App.MediaSvc)); err != nil {
		return fmt.Errorf("register log cleanup cron job: %w", err)
	}
	if err := sc.App.CronScheduler.Register(medialib.NewAutoSyncJob(sc.App.MediaSvc)); err != nil {
		return fmt.Errorf("register media library auto sync cron job: %w", err)
	}
	return nil
}

func startCronSchedulerAction(_ context.Context, sc *StartContext) error {
	sc.App.CronScheduler.Start()
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
