package medialib

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/repository"
)

// newTestDirtyService 给 dirty / sync log 等 kv + 日志场景造一个最小 Service,
// 只关心 db, 不拉 scheduler (Stop / WaitBackground 简化)。
func newTestDirtyService(t *testing.T) *Service {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })
	svc := NewService(sqlite.DB(), t.TempDir(), t.TempDir())
	t.Cleanup(func() {
		svc.Stop()
		svc.WaitBackground()
	})
	return svc
}

// TestIsSyncDirtyDefaultFalse 覆盖边缘 case: yamdc_kv_tab 里没写过 dirty flag
// 时, isSyncDirty 必须返回 false 而不是报错。新装机 / 还没 move 过的用户
// 都走这条路径, 报错会让 scheduler 一直误判需要 sync。
func TestIsSyncDirtyDefaultFalse(t *testing.T) {
	svc := newTestDirtyService(t)
	dirty, err := svc.isSyncDirty(context.Background())
	require.NoError(t, err)
	assert.False(t, dirty)
}

// TestMarkAndClearSyncDirty 覆盖正常 case: mark 之后 isDirty=true,
// clear 之后 isDirty=false。两次 mark 是幂等的, 不应该因为唯一键冲突失败。
func TestMarkAndClearSyncDirty(t *testing.T) {
	svc := newTestDirtyService(t)
	ctx := context.Background()

	require.NoError(t, svc.markSyncDirty(ctx))
	dirty, err := svc.isSyncDirty(ctx)
	require.NoError(t, err)
	assert.True(t, dirty)

	require.NoError(t, svc.markSyncDirty(ctx))
	dirty, err = svc.isSyncDirty(ctx)
	require.NoError(t, err)
	assert.True(t, dirty, "markSyncDirty must be idempotent")

	require.NoError(t, svc.clearSyncDirty(ctx))
	dirty, err = svc.isSyncDirty(ctx)
	require.NoError(t, err)
	assert.False(t, dirty)
}

// TestDirtyFlagNoDB 覆盖异常 case: db == nil 时所有路径都应当退化成 no-op,
// 不 panic 不报错。这对 NewService(nil, ...) 这种 "仅前端 saveDir 场景" 很关键,
// web 层会在配置未生效时传 nil。
func TestDirtyFlagNoDB(t *testing.T) {
	svc := NewService(nil, "", "")
	ctx := context.Background()
	require.NoError(t, svc.markSyncDirty(ctx))
	require.NoError(t, svc.clearSyncDirty(ctx))
	dirty, err := svc.isSyncDirty(ctx)
	require.NoError(t, err)
	assert.False(t, dirty)
}
