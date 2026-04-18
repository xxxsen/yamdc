package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	storage := MustNewSqliteStorage(context.Background(), file)
	if closer, ok := storage.(interface{ Close() error }); ok {
		t.Cleanup(func() {
			require.NoError(t, closer.Close())
		})
	}
	ctx := context.Background()
	// 获取数据, 此时返回错误
	_, err := GetDataFrom(ctx, storage, "abc")
	assert.Error(t, err)
	// 数据不存在
	exist, err := IsDataExistIn(ctx, storage, "abc")
	assert.NoError(t, err)
	assert.False(t, exist)
	// 写入数据
	err = PutDataWithExpireTo(ctx, storage, "abc", []byte("helloworld"), 1*time.Second)
	assert.NoError(t, err)
	// 数据存在
	exist, err = IsDataExistIn(ctx, storage, "abc")
	assert.NoError(t, err)
	assert.True(t, exist)
	// 正常获取数据
	val, err := GetDataFrom(ctx, storage, "abc")
	assert.NoError(t, err)
	assert.Equal(t, "helloworld", string(val))
	// 等待数据过期（避免卡在过期边界导致偶发失败）
	assert.Eventually(t, func() bool {
		exist, err = IsDataExistIn(ctx, storage, "abc")
		assert.NoError(t, err)
		return !exist
	}, 3*time.Second, 50*time.Millisecond)
	_, err = GetDataFrom(ctx, storage, "abc")
	assert.Error(t, err)

	// 测试不过期的数据
	err = PutDataTo(ctx, storage, "zzz", []byte("aaa"))
	assert.NoError(t, err)
	exist, err = IsDataExistIn(ctx, storage, "zzz")
	assert.NoError(t, err)
	assert.True(t, exist)
	val, err = GetDataFrom(ctx, storage, "zzz")
	assert.NoError(t, err)
	assert.Equal(t, "aaa", string(val))
}

func TestStoreCleanupExpiredDeletesExpiredRows(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	storage, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, storage.Close())
	})

	ctx := context.Background()
	require.NoError(t, storage.PutData(ctx, "abc", []byte("helloworld"), 20*time.Millisecond))

	assert.Eventually(t, func() bool {
		require.NoError(t, storage.CleanupExpired(ctx))
		var cnt int
		err := storage.db.QueryRowContext(ctx, "SELECT count(*) FROM cache_tab WHERE key = ?", "abc").Scan(&cnt)
		require.NoError(t, err)
		return cnt == 0
	}, 2*time.Second, 20*time.Millisecond)
}

// ---------- SqliteStorage additional coverage ----------

func TestNewSqliteStorage_Success(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sub", "cache.db")
	s, err := NewSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	if closer, ok := s.(interface{ Close() error }); ok {
		t.Cleanup(func() { _ = closer.Close() })
	}
}

func TestSqliteStore_Close_Nil(t *testing.T) {
	var s *sqliteStore
	assert.NoError(t, s.Close())
}

func TestSqliteStore_Close_NilDB(t *testing.T) {
	s := &sqliteStore{}
	assert.NoError(t, s.Close())
}

func TestSqliteStore_Close_Idempotent(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	assert.NoError(t, s.Close())
}

func TestMustNewSqliteStorage_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustNewSqliteStorage(context.Background(), "/dev/null/impossible/cache.db")
	})
}

func TestConfigureSqliteStoreDB_NilDB(_ *testing.T) {
	configureSqliteStoreDB(context.Background(), nil)
}

func TestSqliteStore_PutReplaceKey(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "k", []byte("v1"), time.Hour))
	require.NoError(t, s.PutData(ctx, "k", []byte("v2"), time.Hour))

	v, err := s.GetData(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, "v2", string(v))
}

func TestSqliteStore_IsDataExist_NotFound(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ok, err := s.IsDataExist(context.Background(), "nonexist")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSqliteStore_GetData_Expired(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "expkey", []byte("v"), 1*time.Millisecond))
	assert.Eventually(t, func() bool {
		_, getErr := s.GetData(ctx, "expkey")
		return getErr != nil
	}, 5*time.Second, 100*time.Millisecond)
}

func TestSqliteStore_IsDataExist_Expired(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "expkey", []byte("v"), 1*time.Millisecond))
	assert.Eventually(t, func() bool {
		ok, existErr := s.IsDataExist(ctx, "expkey")
		return existErr == nil && !ok
	}, 5*time.Second, 100*time.Millisecond)
}

func TestSqliteStore_CleanupExpired_DirectCall(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "expkey", []byte("v"), 1*time.Millisecond))
	assert.Eventually(t, func() bool {
		_, getErr := s.GetData(ctx, "expkey")
		return getErr != nil
	}, 5*time.Second, 100*time.Millisecond)
	require.NoError(t, s.CleanupExpired(ctx))

	var cnt int
	err = s.db.QueryRowContext(ctx, "SELECT count(*) FROM cache_tab WHERE key = ?", "expkey").Scan(&cnt)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt)
}

func TestSqliteStore_Close_MultipleTimes(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	require.NoError(t, s.Close())
}

func TestSqliteStore_PutData_DefaultExpire(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "k", []byte("v"), 0))

	ok, err := s.IsDataExist(ctx, "k")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestSqliteStore_Init_AppliesMigrations(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	var tableName string
	err = s.db.QueryRowContext(context.Background(), "SELECT name FROM sqlite_master WHERE type='table' AND name='cache_tab'").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "cache_tab", tableName)
}

func TestSqliteStore_GetData_Error(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)

	require.NoError(t, s.db.Close())
	_, getErr := s.GetData(context.Background(), "k")
	assert.Error(t, getErr)
}

func TestSqliteStore_PutData_Error(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)

	require.NoError(t, s.db.Close())
	putErr := s.PutData(context.Background(), "k", []byte("v"), time.Hour)
	assert.Error(t, putErr)
}

func TestSqliteStore_IsDataExist_Error(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)

	require.NoError(t, s.db.Close())
	_, existErr := s.IsDataExist(context.Background(), "k")
	assert.Error(t, existErr)
}

func TestSqliteStore_CleanupExpired_Error(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)

	require.NoError(t, s.db.Close())
	err = s.CleanupExpired(context.Background())
	assert.Error(t, err)
}

func TestSqliteStore_Init_AfterTableDrop(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)

	_, execErr := s.db.ExecContext(context.Background(), "DROP TABLE cache_tab")
	require.NoError(t, execErr)
	require.NoError(t, s.init(context.Background()))
	t.Cleanup(func() { _ = s.Close() })
}

func TestNewSqliteStorage_InitFails(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cache.db")

	db, err := openTestDB(file)
	require.NoError(t, err)
	_, _ = db.ExecContext(context.Background(), "CREATE TABLE cache_tab (key TEXT)")
	_, _ = db.ExecContext(context.Background(), "CREATE VIEW idx_expireat AS SELECT 1")
	_ = db.Close()

	_, err = NewSqliteStorage(context.Background(), file)
	require.Error(t, err)
}

func openTestDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

func TestSqliteStore_Close_Error_DoubleClose(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	s, err := newSqliteStorage(context.Background(), file)
	require.NoError(t, err)
	require.NoError(t, s.Close())
}
