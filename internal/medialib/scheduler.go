package medialib

import (
	"context"
	"time"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

// startScheduler 拉起 auto-sync 的后台 goroutine, 包含两条触发线:
//
//  1. 启动时 "opportunistic" 检查: app 起来后 sleep schedulerStartupDelay,
//     读 dirty flag; 为 true 就走一次 full sync。这条覆盖 "机器白天开、
//     凌晨关着, 03:00 永远跑不到" 的用户。
//  2. 每日 03:00 定时检查: 到点读 dirty flag; 为 true 就跑 full sync,
//     否则静默跳过。跳过是本次优化最核心的意义——"无脏状态就不无脑扫盘",
//     对媒体库放 NAS 的场景特别重要, 否则每天 03:00 都要 walk 全库。
//
// scheduler 本身被 shutdownCtx 控制; Stop() 会 cancel 这个 ctx 让它立刻
// 返回。bgWG.Add/Done 让 WaitBackground 能阻塞等它退出。
func (s *Service) startScheduler() {
	if s.db == nil {
		return
	}
	s.bgWG.Add(1)
	go func() {
		defer s.bgWG.Done()
		s.runScheduler(s.shutdownCtx)
	}()
}

func (s *Service) runScheduler(ctx context.Context) {
	logger := logutil.GetLogger(ctx).With(zap.String("component", "media_library_auto_sync"))
	if !s.startupAutoSync(ctx, logger) {
		return
	}
	s.dailyAutoSyncLoop(ctx, logger)
}

// startupAutoSync 执行 "启动延迟 + dirty 触发" 的第一条触发线。返回 false
// 表示 scheduler 应当立刻退出 (ctx 已 cancel, 一般是进程要关了)。
func (s *Service) startupAutoSync(ctx context.Context, logger *zap.Logger) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(s.startupDelay()):
	}
	s.tryAutoSync(ctx, logger, "auto-startup")
	return true
}

func (s *Service) startupDelay() time.Duration {
	if s.schedulerStartupDelay <= 0 {
		return autoSyncStartupDelay
	}
	return s.schedulerStartupDelay
}

// dailyAutoSyncLoop 驱动每日 03:00 定时触发。之所以每轮都重新算一次
// durationUntilNext 而不是用固定 24h Ticker: 一来避免机器 wall-clock
// 跳变 (例如 NTP 修正、手动改时间) 让触发时刻漂走, 二来 schedulerClock
// 在测试里是可注入的, 逐轮重算确保测试能精确断言下一次触发时刻。
func (s *Service) dailyAutoSyncLoop(ctx context.Context, logger *zap.Logger) {
	for {
		d := durationUntilNextDaily(s.now(), autoSyncDailyHour, autoSyncDailyMinute)
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		s.tryAutoSync(ctx, logger, "auto-scheduled")
	}
}

// tryAutoSync 读 dirty flag 判断要不要跑; dirty=false 就只打一行 debug
// 级日志, 不触发任何磁盘 IO。和手动 trigger 走同一入口 (同样的互斥/claim
// /bgWG 语义)。
func (s *Service) tryAutoSync(ctx context.Context, logger *zap.Logger, reason string) {
	dirty, err := s.isSyncDirty(ctx)
	if err != nil {
		logger.Warn("check media library sync dirty flag failed", zap.String("reason", reason), zap.Error(err))
		return
	}
	if !dirty {
		logger.Debug("media library auto sync skipped because dirty flag is clean", zap.String("reason", reason))
		return
	}
	if err := s.triggerFullSyncWithReason(ctx, reason); err != nil {
		logger.Info("media library auto sync not launched",
			zap.String("reason", reason),
			zap.Error(err),
		)
	}
}

func (s *Service) now() time.Time {
	if s.schedulerClock != nil {
		return s.schedulerClock()
	}
	return time.Now()
}

// durationUntilNextDaily 计算距离下一次 (hour:minute) 的时长。如果当前时刻
// 还没过今天的目标点, 返回到今天目标点的间隔; 否则返回到明天目标点的间隔。
// 拆成独立函数是因为这个算式容易写错边界 (15:59:59 vs 16:00:00 vs 16:00:01),
// 独立函数便于在单测里穷举几个 corner case。
func durationUntilNextDaily(now time.Time, hour, minute int) time.Duration {
	today := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !today.After(now) {
		today = today.Add(24 * time.Hour)
	}
	return today.Sub(now)
}
