package medialib

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppendAndListSyncLogs 覆盖正常 case: 写入的几条日志能按时间逆序被
// ListSyncLogs 拿回来, RunID / Level / RelPath / Message 五元组一一对应。
// 逆序是前端 "查看同步日志" 弹窗的默认展示顺序, 回归测必测。
func TestAppendAndListSyncLogs(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	runID := "sync-test-1"
	require.NoError(t, svc.appendSyncLog(ctx, runID, SyncLogLevelInfo, "", "sync start"))
	require.NoError(t, svc.appendSyncLog(ctx, runID, SyncLogLevelWarn, "movie/A", "item warn"))
	require.NoError(t, svc.appendSyncLog(ctx, runID, SyncLogLevelError, "movie/B", "item failed"))

	entries, err := svc.ListSyncLogs(ctx, 0)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	// 逆序展示: 最后一条写入的 "item failed" 排第一。
	assert.Equal(t, SyncLogLevelError, entries[0].Level)
	assert.Equal(t, "movie/B", entries[0].RelPath)
	assert.Equal(t, "item failed", entries[0].Message)
	assert.Equal(t, runID, entries[0].RunID)
	assert.NotZero(t, entries[0].CreatedAt)

	assert.Equal(t, "sync start", entries[2].Message)
}

// TestListSyncLogsLimit 覆盖边缘 case: 传 limit 截断, 不传走默认 200。
// 这里主要验证 limit 穿透到仓储层, 不会因为 medialib 自己加的默认值把
// 明确指定的 limit 覆盖掉。
func TestListSyncLogsLimit(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, svc.appendSyncLog(ctx, "r", SyncLogLevelInfo, "", "x"))
	}

	entries, err := svc.ListSyncLogs(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	entries, err = svc.ListSyncLogs(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, entries, 5)
}

// TestListSyncLogsNoDB 覆盖异常 case: db == nil 时 ListSyncLogs 返回空切片
// 而不是 nil 或报错, 让前端可以直接渲染一个空列表。
func TestListSyncLogsNoDB(t *testing.T) {
	svc := NewService(nil, "", "")
	entries, err := svc.ListSyncLogs(context.Background(), 0)
	require.NoError(t, err)
	assert.NotNil(t, entries)
	assert.Empty(t, entries)
}

// TestCleanupSyncLogsDropsOldRows 覆盖正常 case: cleanup 按 7 天 retention
// 裁旧行。用一条 UPDATE 把第一条日志的 created_at 往回压到很久以前, 再
// 触发 cleanup, 验证裁剪后只剩新鲜那条。
func TestCleanupSyncLogsDropsOldRows(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	require.NoError(t, svc.appendSyncLog(ctx, "old", SyncLogLevelInfo, "", "stale"))
	// 直接改 created_at 到很久以前, 模拟 7 天之前的历史日志。
	_, err := svc.db.ExecContext(ctx,
		`UPDATE yamdc_unified_log_tab SET created_at = 0 WHERE task_id = 'old'`)
	require.NoError(t, err)
	require.NoError(t, svc.appendSyncLog(ctx, "new", SyncLogLevelInfo, "", "fresh"))

	require.NoError(t, svc.cleanupSyncLogs(ctx))

	entries, err := svc.ListSyncLogs(ctx, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "fresh", entries[0].Message)
}

// TestRetentionCutoffMillis 覆盖正常 case: cutoff = now - window, 顺手
// 验证几个典型 window 的数值正确性, 避免未来调 retention 时算错边界。
func TestRetentionCutoffMillis(t *testing.T) {
	now := time.Date(2025, 1, 15, 3, 0, 0, 0, time.UTC)
	cutoff := retentionCutoffMillis(now, 7*24*time.Hour)
	want := now.Add(-7 * 24 * time.Hour).UnixMilli()
	assert.Equal(t, want, cutoff)

	// 0 window 等价于现在: 所有已写入的行都应被裁 (created_at < now)。
	assert.Equal(t, now.UnixMilli(), retentionCutoffMillis(now, 0))
}

// TestNewRunIDFormat 覆盖正常 case: run ID 带 "sync-" 前缀且 hex 部分非空,
// 日志里用肉眼定位时能一眼看出来源。
func TestNewRunIDFormat(t *testing.T) {
	id := newRunID(time.Now())
	assert.Contains(t, id, "sync-")
	assert.Greater(t, len(id), len("sync-"))
}
