// Package cronscheduler 是进程级的全局定时任务编排层, 对上给业务方提供
// "注册一个 Job 就行, 不用自己管 goroutine/ticker/panic 兜底" 的体验,
// 对下包着 github.com/robfig/cron/v3。
//
// 为什么要有这一层 (而不是业务方直接用 robfig/cron):
//
//  1. 统一命名: 每个 Job 暴露 Name(), adapter 会把 name 写进 zap logger
//     的 "cron_job" 字段 (是 zap.Field, 不是 context.Context), 专用于
//     adapter 自身的 started/finished/skipped 日志, 排障时一眼看出 "是
//     哪个 job 在跑/跑了多久/失败原因"。注意这个字段 **不会** 注入到传给
//     Job.Run 的 ctx 里, 业务内部用 logutil.GetLogger(ctx) 拿到的 logger
//     不带 cron_job (详见 jobAdapter 文档)。如果业务方各自用 cron, 日志
//     风格会散成八九种。
//  2. 统一兜底: adapter 层固定包 panic recover + 耗时记录 + SkipIfStillRunning。
//     一个 job panic 不会带挂其它 job, 也不会出现 "前一次 job 还没跑完、
//     下一次 tick 又进来重叠执行" 的怪事。
//  3. 生命周期收敛: Stop() 语义明确 — 拒绝新 tick + 等当前 job 全部返回,
//     带超时保护, 配 bootstrap 的 Cleanup LIFO 能得到可预测的退出顺序。
//  4. 为 "未来更多定时任务" 留口子: scanner / bundle manager / sqlite
//     cache cleanup 这些目前用 ticker 自管的都会慢慢迁进来, 迁的时候
//     实现 Job interface 即可, 不改 scheduler 本体。
package cronscheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Job 是业务方要实现的最小接口。拆三件事:
//
//   - Name: 用于日志/调试; 建议用 snake_case 短语 (e.g. "media_library_auto_sync"),
//     前端不可见, 纯运维视角。不同 Job 的 Name 必须全局唯一, 注册重复会拒绝。
//   - Spec: 接受 robfig/cron 支持的全部语法:
//     标准 5 段 crontab  e.g. "0 3 * * *"        (每天 03:00)
//     @every 简写        e.g. "@every 30s"       (每 30 秒一次)
//     @hourly/@daily 等预设
//     复杂表达式推荐走 crontab, 可读性最好。
//   - Run: 真正的业务逻辑。adapter 会把 Scheduler 的 rootCtx 原样传给
//     Run (不派生子 ctx — 目前没有逐 job 独立取消的诉求, 省一层封装),
//     并在外层包 panic recover + 耗时日志 + SkipIfStillRunning。rootCtx
//     默认由 bootstrap 传 context.Background(), 不会被 Stop 主动 cancel;
//     job 的 "取消" 走 Scheduler.Stop 让 cron 不再调度新 tick, 正在跑的
//     Run 不会被强制打断。长耗时 job 若想响应进程退出, 可自行组合出一
//     个带 cancel 的子 ctx, 但 adapter 这一层不做这件事。
//     返回 error 仅用于 adapter 打日志, 不会影响 scheduler 本身; 想让
//     下一次 tick 直接 skip, 在 Run 里自己判断并返回即可。
type Job interface {
	Name() string
	Spec() string
	Run(ctx context.Context) error
}

// Scheduler 封装 cron.Cron 的生命周期和 Job 注册。非并发安全的构造阶段 +
// 并发安全的运行阶段: Register 只允许在 Start 之前调用, Start 之后锁死;
// 这样避免运行期动态增删 job 带来的竞态 (目前业务也用不到)。
type Scheduler struct {
	cron    *cron.Cron
	logger  *zap.Logger
	rootCtx context.Context

	mu      sync.Mutex
	started bool
	names   map[string]struct{}
}

// Sentinel errors: Register / runWithRecover 返回的所有 error 都 wrap 自
// 这组静态 error, 便于上层用 errors.Is 精准区分原因 (目前 bootstrap 直接
// 打日志不分支, 但保留 wrap 是项目约定, 详见 AGENTS.md 的 err113 规则)。
var (
	errNilJob             = errors.New("cron scheduler: register nil job")
	errEmptyJobName       = errors.New("cron scheduler: job name must not be empty")
	errRegisterAfterStart = errors.New("cron scheduler: cannot register after start")
	errDuplicateJobName   = errors.New("cron scheduler: duplicate job name")
	errInvalidJobSpec     = errors.New("cron scheduler: invalid spec")
	errJobPanic           = errors.New("cron scheduler: job panicked")
	errJobRunFailed       = errors.New("cron scheduler: job run failed")
)

// stopTimeout 是 Scheduler.Stop 等运行中 job 收敛的上限。30 秒是经验值:
// 正常业务 job (auto sync / log cleanup) 要么秒级返回, 要么能响应 ctx cancel
// 在几秒内退出; 设更长没意义, 设更短 (例如 5s) 会让正在收尾刷盘的 job 被
// 强制截断。超时到期后 Stop 会打 warn 直接返回, 不会让进程退出卡住。
const stopTimeout = 30 * time.Second

// New 构造一个 Scheduler。rootCtx 是所有 Job.Run 收到的 ctx, adapter 原样
// 透传 (不派生子 ctx — 目前没有逐 job 独立取消的需求, 省一层封装); 约定
// 由 bootstrap 传进来 (通常是 context.Background, 因为 job 的取消语义走
// Scheduler.Stop 路径, 不走 rootCtx cancel)。logger 用于 adapter 自己的
// 结构化日志。
//
// 使用 time.Local 作为 cron 时区: yamdc 是本地工具, 用户看 "0 3 * * *"
// 期待的是本地时间 03:00, 不是 UTC 03:00。这个选择是有意的, 不要改成 UTC。
func New(rootCtx context.Context, logger *zap.Logger) *Scheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	// 只配 WithLocation; Recover / SkipIfStillRunning 放在 adapter 层自己做,
	// 不用 cron 内置的 chain wrapper. 原因:
	//   (1) cron 内置 Recover 用 cron.Logger 接口, 和项目里的 zap 不兼容,
	//       自己 recover 更直接;
	//   (2) SkipIfStillRunning 用业务层逻辑 (atomic running flag) 更好
	//       debug, 命中时我们可以打带 Name 的 warn。
	c := cron.New(cron.WithLocation(time.Local))
	return &Scheduler{
		cron:    c,
		logger:  logger,
		rootCtx: rootCtx,
		names:   make(map[string]struct{}),
	}
}

// Register 把一个 Job 挂到 scheduler。Start 之后再 Register 会返回错误,
// 避免运行期增删 job 带来的锁住整个 cron 的复杂度 (目前也没这个需求)。
//
// Name 全局唯一; 重复注册是明确错误, 否则日志里两个 job 用同一个 name
// 会让排障极难。
//
// Spec 解析失败 (语法错误) 也在这里抛出, 不要推迟到第一次 tick 才炸。
func (s *Scheduler) Register(j Job) error {
	if j == nil {
		return errNilJob
	}
	name := j.Name()
	if name == "" {
		return errEmptyJobName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return fmt.Errorf("%w: %q", errRegisterAfterStart, name)
	}
	if _, dup := s.names[name]; dup {
		return fmt.Errorf("%w: %q", errDuplicateJobName, name)
	}
	adapter := newJobAdapter(s.rootCtx, j, s.logger)
	if _, err := s.cron.AddJob(j.Spec(), adapter); err != nil {
		return fmt.Errorf("%w: job=%q spec=%q: %w", errInvalidJobSpec, name, j.Spec(), err)
	}
	s.names[name] = struct{}{}
	s.logger.Info("cron job registered",
		zap.String("cron_job", name),
		zap.String("spec", j.Spec()),
	)
	return nil
}

// Start 拉起 cron 主 goroutine。幂等: 重复调用是 no-op, 不报错, 方便 bootstrap
// 在复杂启停场景下不需要记住 "是否已经 Start 过"。
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	jobCount := len(s.names)
	s.mu.Unlock()
	s.cron.Start()
	s.logger.Info("cron scheduler started", zap.Int("job_count", jobCount))
}

// Stop 让 cron 停止调度新的 tick, 等正在运行的 job 最长 stopTimeout 返回。
// 幂等: 没 Start 过或重复 Stop 都是 no-op。
//
// 返回前要么所有 job 都返回 (正常退出), 要么到达 stopTimeout (打 warn 后
// 放弃等)。到达超时不算错误: 进程整体还要继续走 cleanup 链, 后面的
// cancel/wait 能兜底清理卡住的 goroutine。
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	s.mu.Unlock()
	s.logger.Info("cron scheduler stopping")
	stopCtx := s.cron.Stop()
	select {
	case <-stopCtx.Done():
		s.logger.Info("cron scheduler stopped, all jobs finished")
	case <-time.After(stopTimeout):
		s.logger.Warn("cron scheduler stop timed out, running jobs will be abandoned",
			zap.Duration("timeout", stopTimeout),
		)
	}
}

// jobAdapter 把 Job 适配到 cron.Job (Run() 无参), 负责:
//   - 持有一个带 cron_job=<name> 字段的 zap logger, 用于 adapter 自身打
//     started/finished/skipped/panic 这几条结构化日志; 注意 **这个 logger
//     不会注入到传给 Job.Run 的 ctx**, 业务代码里调 logutil.GetLogger(ctx)
//     仍然拿到全局 logger (xxxsen/common/logutil 只从 ctx 取 traceid, 不认
//     识别的 context value), 业务日志要带 cron 相关字段需自己挂;
//   - 记录开始 / 结束 / 耗时 / 错误;
//   - panic recover: 打 error 后吞掉, 保证其它 job 继续跑;
//   - SkipIfStillRunning: 上一次 Run 没返回前, 下一次 tick 直接 skip
//     并打 warn。用 atomic CompareAndSwap 足够轻。
type jobAdapter struct {
	job     Job
	rootCtx context.Context
	logger  *zap.Logger

	// running 用 atomic.Int32 做锁旗: 0=idle, 1=running。比 sync.Mutex
	// 好的地方是 "发现正在跑就直接 skip" 而不是阻塞等锁 — cron tick 要是
	// 阻塞住, 一堆 tick 积压最终会让调度彻底乱掉。
	running atomic.Int32
}

func newJobAdapter(rootCtx context.Context, j Job, logger *zap.Logger) *jobAdapter {
	return &jobAdapter{
		job:     j,
		rootCtx: rootCtx,
		logger:  logger.With(zap.String("cron_job", j.Name())),
	}
}

// Run 实现 cron.Job. cron.Job 签名没有 ctx, adapter 把 Scheduler 的 rootCtx
// 原样透传给 Job.Run (见 runWithRecover)。绝大部分定时任务的 "取消" 不靠
// ctx, 而是靠 Scheduler.Stop 让 cron 不再调度新 tick; rootCtx 默认是
// context.Background, 进程退出时 **不会** 被 Stop 主动 cancel, 正在跑的
// Job.Run 不会因此中断 (这是刻意取舍, 见 Job 接口的文档)。
func (a *jobAdapter) Run() {
	if !a.running.CompareAndSwap(0, 1) {
		a.logger.Warn("cron job skipped: previous run still in progress")
		return
	}
	defer a.running.Store(0)

	start := time.Now()
	a.logger.Info("cron job started")

	err := a.runWithRecover()

	duration := time.Since(start)
	switch {
	case err != nil:
		a.logger.Warn("cron job finished with error",
			zap.Duration("duration", duration),
			zap.Error(err),
		)
	default:
		a.logger.Info("cron job finished",
			zap.Duration("duration", duration),
		)
	}
}

// runWithRecover 把 panic 翻译成 error, 避免 adapter 自身因 panic 退出
// cron 的 goroutine pool。recover 不 rethrow 是刻意的: 定时任务的单次
// panic 不该让整个进程挂掉, 打 error 已经足够报警。
//
// 返回的 error 统一 wrap 自 errJobPanic / errJobRunFailed, 调用方 (adapter.Run)
// 只用来打日志, 不做分支处理; wrap 是为了满足项目 err113 / wrapcheck 约束。
func (a *jobAdapter) runWithRecover() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", errJobPanic, r)
		}
	}()
	if runErr := a.job.Run(a.rootCtx); runErr != nil {
		return fmt.Errorf("%w: %w", errJobRunFailed, runErr)
	}
	return nil
}
