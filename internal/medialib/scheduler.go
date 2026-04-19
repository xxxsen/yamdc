package medialib

import (
	"context"
	"fmt"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/cronscheduler"
)

// 1.5 起 media library 的定时任务从 Service 内自管的 goroutine + Timer
// 挪到 internal/cronscheduler 统一调度。Service 不再持有 scheduler 相关
// 字段 (schedulerClock / schedulerStartupDelay 已删), 只对 cron scheduler
// 暴露 "可注册的 Job" 工厂。
//
// 对应的两个 Job:
//
//   - AutoSyncJob:    每日 03:00 检查 dirty flag, 为 true 就跑 full sync;
//                     dirty=false 静默 skip, 不触发任何磁盘 IO。
//   - LogCleanupJob:  每日 03:00 裁 7 天外的日志。和 AutoSyncJob 独立的原因:
//                     "用户只刮不入库 -> sync 从不跑 -> cleanup 从不跑
//                     -> 日志无限累积" 这条 bug 路径必须避免 (见 log_repository
//                     里 DeleteOlderThan 的注释)。两个 Job 都注册, 即使 sync
//                     被 skip, cleanup 也照跑。
//
// 之前的 "startupAutoSync" (进程起来后 60s 再跑一次 dirty 检查) 在 1.5 里
// 直接砍掉: 引入 cron scheduler 后, "startup 触发" 和 "daily tick" 的并发
// 语义难管 (可能并发读 dirty + 并发调 triggerFullSyncWithReason), 而收益
// 仅针对 "机器长时间关机" 的少数用户 — 他们进 UI 手动点同步也能触发,
// 并没有丢功能的紧迫性。

// autoSyncJobName / logCleanupJobName 供 cron adapter 作为 zap 的 cron_job
// 字段值 (只在 adapter 自己的 started/finished/skipped 日志上, 不注入业务
// 的 logutil.GetLogger(ctx)), 排障时能 filter 某一类 job 的所有历史触发。
// 保持 snake_case 和项目其他结构化字段一致。
const (
	autoSyncJobName   = "media_library_auto_sync"
	logCleanupJobName = "media_library_log_cleanup"
	// autoSyncReason 也写进 task_state_tab.message 区分触发来源, 和手动
	// 触发 ("manual") 对立。1.5 砍掉 startup 触发线后, 自动 sync 只剩
	// cron 一路, 所以不再有多种 reason 枚举。
	autoSyncReason = "auto-scheduled"
)

// NewAutoSyncJob 返回每日 03:00 跑的 "dirty 时触发 sync" Job。
// 注册时期传给 cronscheduler.Scheduler.Register。Job 实现对 svc 是一个引用,
// 被 cron 调度时直接调 svc 的非导出方法 (tryAutoSync), 不用把它升成 public
// 污染 Service 的外部 API。
func NewAutoSyncJob(svc *Service) cronscheduler.Job {
	return &autoSyncJob{svc: svc}
}

// NewLogCleanupJob 返回每日 03:00 跑的日志 retention 清理 Job。
// cleanup 不依赖 dirty flag: 只要进程活着, 日志表就会每天被裁一次。
func NewLogCleanupJob(svc *Service) cronscheduler.Job {
	return &logCleanupJob{svc: svc}
}

type autoSyncJob struct {
	svc *Service
}

func (j *autoSyncJob) Name() string { return autoSyncJobName }
func (j *autoSyncJob) Spec() string { return AutoSyncCronSpec }
func (j *autoSyncJob) Run(ctx context.Context) error {
	// ctx 来自 cron adapter, 但 adapter 的 cron_job 字段 logger 只挂在
	// adapter 自己的 started/finished 日志上 — ctx 本身不携带它
	// (xxxsen/common/logutil 只从 ctx 取 traceid)。这里通过 logutil.GetLogger
	// 拿到的是全局 logger, 由 tryAutoSync 自己补 reason 等业务字段, 不会带
	// cron_job。需要 cron 维度排障时用 adapter 外层日志 + task_state_tab.message
	// 里的 autoSyncReason 关联即可。
	j.svc.tryAutoSync(ctx, logutil.GetLogger(ctx))
	return nil
}

type logCleanupJob struct {
	svc *Service
}

func (j *logCleanupJob) Name() string { return logCleanupJobName }
func (j *logCleanupJob) Spec() string { return LogCleanupCronSpec }
func (j *logCleanupJob) Run(ctx context.Context) error {
	if err := j.svc.cleanupSyncLogs(ctx); err != nil {
		return fmt.Errorf("cleanup media library logs: %w", err)
	}
	return nil
}

// tryAutoSync 读 dirty flag 判断要不要跑; dirty=false 就只打一行 debug
// 级日志, 不触发任何磁盘 IO。和手动 trigger 走同一入口 (同样的互斥 /
// claim / bgWG 语义), 所以不会和 move 任务、也不会和另一条同期触发的
// auto sync 并发。
//
// reason 硬编码 autoSyncReason 而不作为参数: 1.5 砍 startup 触发线之后
// 只剩一路调用 (AutoSyncJob.Run), 参数化 reason 会被 lint (unparam) 判
// "永远传同一个值"; 后续如果要加新触发源, 再把 reason 重新参数化即可。
//
// 上层 (AutoSyncJob.Run) 从不检查本函数的错误: 所有分支都内部打日志,
// 调用方只关心 "tick 是否跑完"。返回 void 也是故意的。
func (s *Service) tryAutoSync(ctx context.Context, logger *zap.Logger) {
	dirty, err := s.isSyncDirty(ctx)
	if err != nil {
		logger.Warn("check media library sync dirty flag failed",
			zap.String("reason", autoSyncReason), zap.Error(err))
		return
	}
	if !dirty {
		logger.Debug("media library auto sync skipped because dirty flag is clean",
			zap.String("reason", autoSyncReason))
		return
	}
	if err := s.triggerFullSyncWithReason(ctx, autoSyncReason); err != nil {
		logger.Info("media library auto sync not launched",
			zap.String("reason", autoSyncReason),
			zap.Error(err),
		)
	}
}
