package medialib

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/repository"
	"go.uber.org/zap"
)

// TestTryAutoSyncSkipsWhenClean 覆盖正常 case: dirty flag 为 false 时
// tryAutoSync 不应该触发 full sync (task_state_tab 里 sync 状态不应出现)。
// 这是整个优化 "无脏不扫盘" 的核心不变量。
func TestTryAutoSyncSkipsWhenClean(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	svc.tryAutoSync(ctx, zap.NewNop())

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

	svc.tryAutoSync(ctx, zap.NewNop())

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
	svc.tryAutoSync(context.Background(), zap.NewNop())
}

// TestAutoSyncJobMetadata 覆盖正常 case: Name 全局唯一, Spec 是每日 03:00
// crontab 表达式。Name 漂移会让运行期日志过滤失败; Spec 漂移会让触发时间
// 偏离设计, 所以两个都做字面量断言 (回归重构时必须显式意识到这里在变)。
func TestAutoSyncJobMetadata(t *testing.T) {
	job := NewAutoSyncJob(nil)
	assert.Equal(t, "media_library_auto_sync", job.Name())
	assert.Equal(t, "0 3 * * *", job.Spec())
}

// TestAutoSyncJobRunDelegatesToTryAutoSync 覆盖正常 case: Run 把 "dirty
// 触发 sync" 这条路径接到 cron adapter。dirty=true 时 Run 返回 nil (job
// 不应把 trigger 失败翻成 error - 所有分支都是内部日志), sync 被启动,
// dirty 被清。这个是 Job 层能不能真正触发 sync 的端到端断言。
func TestAutoSyncJobRunDelegatesToTryAutoSync(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	require.NoError(t, svc.markSyncDirty(ctx))

	job := NewAutoSyncJob(svc)
	require.NoError(t, job.Run(ctx))

	svc.WaitBackground()

	dirty, err := svc.isSyncDirty(ctx)
	require.NoError(t, err)
	assert.False(t, dirty)
}

// TestLogCleanupJobMetadata 覆盖正常 case: 和 AutoSync 一样做 Name/Spec
// 断言, 防止改动时漂掉。
//
// 注意 Spec 刻意是 "15 3 * * *" 而不是 "0 3 * * *": 和 AutoSync 错开 15
// 分钟执行, 避免两个 03:00 整点的 job 在同一 tick 里并发抢 sqlite 锁 —
// cron adapter 会按注册顺序依次触发, 但各 Job.Run 之间相互不阻塞, 错开
// 比 "靠 SkipIfStillRunning 兜底" 更稳。改 Spec 要一并改这里的断言。
func TestLogCleanupJobMetadata(t *testing.T) {
	job := NewLogCleanupJob(nil)
	assert.Equal(t, "media_library_log_cleanup", job.Name())
	assert.Equal(t, "15 3 * * *", job.Spec())
}

// TestLogCleanupJobRunScrubsOldRows 覆盖正常 case: 写一条日志 + 手动改
// created_at 到 7 天前, 跑一次 LogCleanupJob, 旧行被裁、新行保留。
// 模拟 "用户一直刮不入库, cleanup 不依赖 sync 也能独立跑" 这条 1.5 里
// 关键修复路径。
func TestLogCleanupJobRunScrubsOldRows(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	require.NoError(t, svc.appendSyncLog(ctx, "old", SyncLogLevelInfo, "", "stale"))
	_, err := svc.db.ExecContext(ctx,
		`UPDATE yamdc_unified_log_tab SET created_at = 0 WHERE task_id = 'old'`)
	require.NoError(t, err)
	require.NoError(t, svc.appendSyncLog(ctx, "new", SyncLogLevelInfo, "", "fresh"))

	job := NewLogCleanupJob(svc)
	require.NoError(t, job.Run(ctx))

	entries, err := svc.ListSyncLogs(ctx, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "fresh", entries[0].Message)
}

// TestLogCleanupJobRunNoDB 覆盖边缘 case: Service 没配 db 时 Run 不 panic
// 也不返回 error, 和其它 no-db 路径保持一致语义。
func TestLogCleanupJobRunNoDB(t *testing.T) {
	svc := NewService(nil, "/lib", "")
	job := NewLogCleanupJob(svc)
	assert.NoError(t, job.Run(context.Background()))
	// 纯防御: 不该起任何 goroutine。
	done := make(chan struct{})
	go func() {
		svc.WaitBackground()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("log cleanup job with nil db should not start goroutines")
	}
}

// TestLogCleanupJobRunPropagatesRepoError 覆盖异常 case: 底层 DeleteOlderThan
// 返回 error 时, Run 必须把错误原样 wrap 向上抛, 让 cron adapter 能记录
// "finished with error" 而不是吞掉。吞错会让运维以为 retention 一直在跑,
// 实际日志表早已停止裁剪。
//
// 构造手法: 建完 Service 后把底层 sqlite 连接关掉, 这样 DeleteOlderThan
// 会必然走进 "DB closed" 分支返回 error。这是目前最稳的不用 mock 的触发
// 方式 (LogRepository 是具体类型, 不是接口, 无法直接注入 fake)。
func TestLogCleanupJobRunPropagatesRepoError(t *testing.T) {
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	svc := NewService(sqlite.DB(), t.TempDir(), t.TempDir())
	t.Cleanup(func() {
		svc.Stop()
		svc.WaitBackground()
	})

	require.NoError(t, sqlite.Close(), "close underlying sqlite to force DeleteOlderThan error")

	job := NewLogCleanupJob(svc)
	err = job.Run(context.Background())
	require.Error(t, err, "closed DB must surface as error from Run")
	assert.Contains(t, err.Error(), "cleanup media library logs",
		"error must be wrapped with Job-level prefix for log filtering")
}
