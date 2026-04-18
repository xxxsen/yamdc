package medialib

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestDurationUntilNextDaily 覆盖 3 种 case: 还没到目标时刻 -> 今天 (正常),
// 刚好等于目标时刻 -> 明天 (边缘, "不触发今天是为了避免定时器漂移反复触发"),
// 已经过了目标时刻 -> 明天 (异常场景, 比如时钟跳变/机器刚开机)。
// 这三条路径正好覆盖 durationUntilNextDaily 里的所有分支。
func TestDurationUntilNextDaily(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{
			name: "before target same day",
			now:  time.Date(2025, 6, 1, 1, 30, 0, 0, loc),
			want: 90 * time.Minute,
		},
		{
			name: "exactly target goes to next day",
			now:  time.Date(2025, 6, 1, 3, 0, 0, 0, loc),
			want: 24 * time.Hour,
		},
		{
			name: "after target rolls next day",
			now:  time.Date(2025, 6, 1, 12, 0, 0, 0, loc),
			want: 15 * time.Hour,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := durationUntilNextDaily(tc.now, 3, 0)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestStartupDelayFallsBackToDefault 覆盖边缘 case: schedulerStartupDelay <= 0
// 必须走 autoSyncStartupDelay 默认值, 不能返回 0 让 goroutine 立刻跑。
func TestStartupDelayFallsBackToDefault(t *testing.T) {
	svc := newTestDirtyService(t)
	svc.schedulerStartupDelay = 0
	assert.Equal(t, autoSyncStartupDelay, svc.startupDelay())

	svc.schedulerStartupDelay = -time.Second
	assert.Equal(t, autoSyncStartupDelay, svc.startupDelay())

	svc.schedulerStartupDelay = 123 * time.Millisecond
	assert.Equal(t, 123*time.Millisecond, svc.startupDelay())
}

// TestNowUsesSchedulerClock 覆盖边缘 case: schedulerClock 注入时走注入的 clock;
// 为 nil 时退化到 time.Now, 用 "差值 < 1s" 做一个宽松的断言避免 flaky。
func TestNowUsesSchedulerClock(t *testing.T) {
	svc := newTestDirtyService(t)
	fixed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.schedulerClock = func() time.Time { return fixed }
	assert.Equal(t, fixed, svc.now())

	svc.schedulerClock = nil
	assert.WithinDuration(t, time.Now(), svc.now(), time.Second)
}

// TestTryAutoSyncSkipsWhenClean 覆盖正常 case: dirty flag 为 false 时
// tryAutoSync 不应该触发 full sync (task_state_tab 里 sync 状态不应出现)。
// 这是整个优化 "无脏不扫盘" 的核心不变量。
func TestTryAutoSyncSkipsWhenClean(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	svc.tryAutoSync(ctx, zap.NewNop(), "auto-scheduled")

	state, err := svc.getTaskState(ctx, TaskSync)
	require.NoError(t, err)
	assert.Equal(t, "idle", state.Status, "dirty=false must not start a sync task state")
}

// TestTryAutoSyncTriggersWhenDirty 覆盖正常 case: dirty=true 时 tryAutoSync
// 应该触发一次 full sync, sync 跑完后 dirty 被清零 (runFullSync 的 defer
// clearSyncDirty 的保护语义)。用 WaitBackground 阻塞等待 sync 完成。
func TestTryAutoSyncTriggersWhenDirty(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	require.NoError(t, svc.markSyncDirty(ctx))

	svc.tryAutoSync(ctx, zap.NewNop(), "auto-scheduled")

	// runFullSync 是 goroutine, 阻塞等它跑完再断言。libraryDir 是空 TempDir,
	// sync 本身走完全部流程但没有实际文件被处理, 秒级完成。
	svc.WaitBackground()

	dirty, err := svc.isSyncDirty(ctx)
	require.NoError(t, err)
	assert.False(t, dirty, "runFullSync must clear dirty flag on finalize")

	state, err := svc.getTaskState(ctx, TaskSync)
	require.NoError(t, err)
	assert.Equal(t, "completed", state.Status, "sync must finish successfully on empty library")

	logs, err := svc.ListSyncLogs(ctx, 0)
	require.NoError(t, err)
	require.NotEmpty(t, logs, "sync start/end logs must be persisted")
}

// TestTryAutoSyncNoDB 覆盖异常 case: Service 没配 db 时 tryAutoSync 不 panic,
// 只做一次 isSyncDirty 检查 (返回 false) 然后安静返回。
func TestTryAutoSyncNoDB(_ *testing.T) {
	svc := NewService(nil, "/lib", "")
	svc.tryAutoSync(context.Background(), zap.NewNop(), "auto-startup")
}

// TestRunSchedulerStartupTriggers 覆盖正常 case: startupDelay 压到极短, 标 dirty,
// runScheduler 通过 startupAutoSync 触发一次 sync。dailyAutoSyncLoop 的
// 下一轮 timer 被 shutdownCtx cancel 打断, goroutine 干净退出。
func TestRunSchedulerStartupTriggers(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	require.NoError(t, svc.markSyncDirty(ctx))

	// 把 startup delay 压到 1ms, 让测试不必真的等 60s。schedulerClock 保持
	// 默认 time.Now 让 dailyAutoSyncLoop 的 timer 走到一个 "很久以后" 的
	// 触发点, 之后被 shutdownCtx cancel 打断。
	svc.schedulerStartupDelay = time.Millisecond

	svc.startScheduler()

	// scheduler 启动后, startup 触发的 sync 是异步的, 等 dirty 被清掉
	// (runFullSync 的 finalize) 作为成功信号。
	require.Eventually(t, func() bool {
		dirty, err := svc.isSyncDirty(ctx)
		return err == nil && !dirty
	}, 5*time.Second, 20*time.Millisecond, "dirty flag should be cleared by auto sync")

	svc.Stop()
	svc.WaitBackground()
}

// TestRunSchedulerCanceledBeforeStartup 覆盖边缘 case: 进程在 startup delay
// 期间就被 Stop 掉 (用户启动 / 立刻关机), scheduler 必须立刻返回不跑 sync;
// dirty flag 不应该被意外清掉。
func TestRunSchedulerCanceledBeforeStartup(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	require.NoError(t, svc.markSyncDirty(ctx))
	// 把 delay 压到 50ms, 启动后立刻 Stop, 让 ctx.Done 分支先命中。
	svc.schedulerStartupDelay = 50 * time.Millisecond
	svc.startScheduler()
	svc.Stop()
	svc.WaitBackground()

	dirty, err := svc.isSyncDirty(ctx)
	require.NoError(t, err)
	assert.True(t, dirty, "scheduler canceled before startup must not trigger sync")
}

// TestStartSchedulerNoDB 覆盖异常 case: 没配 db 时 startScheduler 必须直接
// 返回, 不应该拉起任何 goroutine (否则 bgWG 会 hang 住 WaitBackground)。
func TestStartSchedulerNoDB(t *testing.T) {
	svc := NewService(nil, "/lib", "")
	svc.startScheduler()
	done := make(chan struct{})
	go func() {
		svc.WaitBackground()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WaitBackground blocked, startScheduler should no-op when db is nil")
	}
}
