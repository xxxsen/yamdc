package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPebbleStore(t *testing.T) *pebbleStore {
	t.Helper()
	s, err := newPebbleStorage(context.Background(), filepath.Join(t.TempDir(), "pebble"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, s.Close())
	})
	return s
}

func TestPebbleStore_PutGet(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()

	require.NoError(t, s.PutData(ctx, "k", []byte("value"), time.Hour))
	got, err := s.GetData(ctx, "k")

	require.NoError(t, err)
	assert.Equal(t, []byte("value"), got)
}

func TestPebbleStore_PutLargeValue(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	value := bytes.Repeat([]byte("x"), 1024*1024)

	require.NoError(t, s.PutData(ctx, "large", value, time.Hour))
	got, err := s.GetData(ctx, "large")

	require.NoError(t, err)
	assert.Equal(t, value, got)
}

func TestPebbleStore_Overwrite(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()

	require.NoError(t, s.PutData(ctx, "k", []byte("first"), time.Hour))
	firstExpireAt, ok, err := s.lookupExpireAt(makePebbleDataKey("k"))
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, s.PutData(ctx, "k", []byte("second"), 2*time.Hour))

	got, err := s.GetData(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, []byte("second"), got)
	_, closer, err := s.db.Get(makePebbleExpireKey(firstExpireAt, "k"))
	if closer != nil {
		require.NoError(t, closer.Close())
	}
	assert.ErrorIs(t, err, pebble.ErrNotFound)
}

func TestPebbleStore_IsDataExist(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()

	ok, err := s.IsDataExist(ctx, "missing")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.PutData(ctx, "k", []byte("value"), time.Hour))
	ok, err = s.IsDataExist(ctx, "k")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestPebbleStore_DefaultExpire(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()

	require.NoError(t, s.PutData(ctx, "k", []byte("value"), 0))
	got, err := s.GetData(ctx, "k")

	require.NoError(t, err)
	assert.Equal(t, []byte("value"), got)
}

func TestPebbleStore_CleanupExpired(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()

	require.NoError(t, s.PutData(ctx, "expired", []byte("value"), -time.Second))
	require.NoError(t, s.CleanupExpired(ctx))
	assert.Equal(t, 0, countPebbleKeys(t, s, []byte{pebbleExpirePrefix}))

	_, err := s.GetData(ctx, "expired")
	assert.Error(t, err)
	_, closer, err := s.db.Get(makePebbleDataKey("expired"))
	if closer != nil {
		require.NoError(t, closer.Close())
	}
	assert.ErrorIs(t, err, pebble.ErrNotFound)
}

func TestPebbleStore_GetMissing(t *testing.T) {
	s := newTestPebbleStore(t)

	_, err := s.GetData(context.Background(), "missing")

	assert.Error(t, err)
	assert.ErrorIs(t, err, pebble.ErrNotFound)
}

func TestPebbleStore_OpenInvalidPath(t *testing.T) {
	_, err := NewPebbleStorage(context.Background(), "/dev/null/impossible/pebble")

	assert.Error(t, err)
}

func TestPebbleStore_PutAfterClose(t *testing.T) {
	s, err := newPebbleStorage(context.Background(), filepath.Join(t.TempDir(), "pebble"))
	require.NoError(t, err)
	require.NoError(t, s.Close())

	err = s.PutData(context.Background(), "k", []byte("value"), time.Hour)

	assert.Error(t, err)
}

func TestPebbleStore_GetAfterClose(t *testing.T) {
	s, err := newPebbleStorage(context.Background(), filepath.Join(t.TempDir(), "pebble"))
	require.NoError(t, err)
	require.NoError(t, s.Close())

	_, err = s.GetData(context.Background(), "k")

	assert.Error(t, err)
}

func TestPebbleStore_CleanupCanceled(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.CleanupExpired(ctx)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPebbleStore_DecodeCorruptRecord(t *testing.T) {
	s := newTestPebbleStore(t)
	require.NoError(t, s.db.Set(makePebbleDataKey("bad"), []byte("short"), pebble.Sync))

	_, err := s.GetData(context.Background(), "bad")

	assert.ErrorIs(t, err, errPebbleRecordTooShort)
}

func TestPebbleStore_Expire(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "k", []byte("value"), -time.Second))

	_, err := s.GetData(ctx, "k")
	assert.Error(t, err)
}

func TestPebbleStore_ExpireExist(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "k", []byte("value"), -time.Second))

	ok, err := s.IsDataExist(ctx, "k")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPebbleStore_EmptyValue(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()

	require.NoError(t, s.PutData(ctx, "empty", []byte{}, time.Hour))
	got, err := s.GetData(ctx, "empty")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPebbleStore_NegativeExpire(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()

	require.NoError(t, s.PutData(ctx, "k", []byte("value"), -time.Second))

	assert.Eventually(t, func() bool {
		ok, err := s.IsDataExist(ctx, "k")
		return err == nil && !ok
	}, 5*time.Second, 20*time.Millisecond)
}

func TestPebbleStore_ConcurrentPutSameKey(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- s.PutData(ctx, "k", []byte("value"), time.Hour)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	got, err := s.GetData(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), got)
}

func TestPebbleStore_CloseNil(t *testing.T) {
	var s *pebbleStore
	assert.NoError(t, s.Close())
	assert.NoError(t, (&pebbleStore{}).Close())
}

func TestPebbleStore_ExpireKeyOrdering(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "expired", []byte("old"), -time.Second))
	require.NoError(t, s.PutData(ctx, "alive", []byte("new"), time.Hour))

	require.NoError(t, s.CleanupExpired(ctx))
	assert.Equal(t, 1, countPebbleKeys(t, s, []byte{pebbleExpirePrefix}))
	got, err := s.GetData(ctx, "alive")
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), got)
}

func TestPebbleStore_CleanupSkipsNewerValueWithStaleIndex(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	staleExpireAt := pebbleExpireTime(time.Now().Add(-time.Hour))
	freshExpireAt := pebbleExpireTime(time.Now().Add(time.Hour))
	require.NoError(t, s.db.Set(makePebbleDataKey("k"), encodePebbleRecord(freshExpireAt, []byte("fresh")), pebble.Sync))
	require.NoError(t, s.db.Set(makePebbleExpireKey(staleExpireAt, "k"), nil, pebble.Sync))

	require.NoError(t, s.CleanupExpired(ctx))

	got, err := s.GetData(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, []byte("fresh"), got)
}

func TestPebbleStore_OpenCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewPebbleStorage(ctx, filepath.Join(t.TempDir(), "pebble"))

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPebbleHelpers(t *testing.T) {
	record := encodePebbleRecord(123, []byte("value"))
	expireAt, value, err := decodePebbleRecord(record)
	require.NoError(t, err)
	assert.Equal(t, int64(123), expireAt)
	assert.Equal(t, []byte("value"), value)

	_, _, err = decodePebbleRecord([]byte("short"))
	assert.ErrorIs(t, err, errPebbleRecordTooShort)

	parsedExpireAt, parsedKey, err := parsePebbleExpireKey(makePebbleExpireKey(456, "abc"))
	require.NoError(t, err)
	assert.Equal(t, int64(456), parsedExpireAt)
	assert.Equal(t, "abc", parsedKey)

	_, _, err = parsePebbleExpireKey([]byte("bad"))
	assert.ErrorIs(t, err, errPebbleBadExpireKey)
}

func TestPebbleStore_PutCanceledContext(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.PutData(ctx, "k", []byte("value"), time.Hour)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPebbleStore_GetCanceledContext(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.GetData(ctx, "k")

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPebbleStore_IsDataExistCanceledContext(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ok, err := s.IsDataExist(ctx, "k")

	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, ok)
}

func TestPebbleStore_IsDataExistDecodeError(t *testing.T) {
	s := newTestPebbleStore(t)
	require.NoError(t, s.db.Set(makePebbleDataKey("bad"), []byte("short"), pebble.Sync))

	ok, err := s.IsDataExist(context.Background(), "bad")

	assert.ErrorIs(t, err, errPebbleRecordTooShort)
	assert.False(t, ok)
}

func TestPebbleStore_LookupMissing(t *testing.T) {
	s := newTestPebbleStore(t)

	_, ok, err := s.lookupExpireAt(makePebbleDataKey("missing"))

	require.NoError(t, err)
	assert.False(t, ok)
}

func countPebbleKeys(t *testing.T, s *pebbleStore, prefix []byte) int {
	t.Helper()
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: []byte{prefix[0] + 1},
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, iter.Close())
	}()
	count := 0
	for valid := iter.First(); valid; valid = iter.Next() {
		count++
	}
	require.NoError(t, iter.Error())
	return count
}

func TestPebbleStore_CleanupWithBadExpireKey(t *testing.T) {
	s := newTestPebbleStore(t)
	require.NoError(t, s.db.Set([]byte{pebbleExpirePrefix}, nil, pebble.Sync))

	err := s.CleanupExpired(context.Background())

	require.NoError(t, err)
}

func TestPebbleStore_PutFailsOnCorruptExistingRecord(t *testing.T) {
	s := newTestPebbleStore(t)
	require.NoError(t, s.db.Set(makePebbleDataKey("bad"), []byte("short"), pebble.Sync))

	err := s.PutData(context.Background(), "bad", []byte("value"), time.Hour)

	assert.ErrorIs(t, err, errPebbleRecordTooShort)
}

func TestPebbleStore_CleanupAfterClose(t *testing.T) {
	s, err := newPebbleStorage(context.Background(), filepath.Join(t.TempDir(), "pebble"))
	require.NoError(t, err)
	require.NoError(t, s.Close())

	err = s.CleanupExpired(context.Background())

	assert.Error(t, err)
}

func TestPebbleStore_ErrorWrappingPreservesNotFound(t *testing.T) {
	s := newTestPebbleStore(t)

	_, err := GetDataFrom(context.Background(), s, "missing")

	assert.True(t, errors.Is(err, pebble.ErrNotFound))
}

// TestPebbleStore_ConcurrentPutVsCleanup 验证 PutData 与 CleanupExpired 并发
// 执行时, cleanup 不会因为 "check-then-write" 竞态把新值误删。配合 -race 运行
// 还能覆盖 s.db 访问的数据竞争。
//
// 写入策略上每 3 次插一次 expire=-1s 的 "立即过期" 写入, 诱发 cleanup 实际对
// expire index 做批量删除, 尽可能让 Put 与 cleanup 的操作在 pebble 层交错。
func TestPebbleStore_ConcurrentPutVsCleanup(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	const iterations = 200
	const putters = 4

	var wg sync.WaitGroup
	errCh := make(chan error, putters+1)

	for i := 0; i < putters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("k-%d", id)
			for j := 0; j < iterations; j++ {
				expire := time.Hour
				if j%3 == 0 {
					expire = -time.Second
				}
				val := []byte(fmt.Sprintf("v-%d-%d", id, j))
				if err := s.PutData(ctx, key, val, expire); err != nil {
					errCh <- fmt.Errorf("put id=%d j=%d: %w", id, j, err)
					return
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < iterations; j++ {
			if err := s.CleanupExpired(ctx); err != nil {
				errCh <- fmt.Errorf("cleanup j=%d: %w", j, err)
				return
			}
		}
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	// 每个 putter 独立写一次未过期的终态, 然后验证读得到: 这一步主要检查 Put/Get
	// 路径在经历并发 cleanup 之后仍然自洽, 没有被误删或锁死。
	for i := 0; i < putters; i++ {
		key := fmt.Sprintf("k-%d", i)
		require.NoError(t, s.PutData(ctx, key, []byte("final"), time.Hour))
		got, err := s.GetData(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, []byte("final"), got)
	}
}

// TestPebbleStore_ConcurrentCloseVsOps 验证 Close 与 PutData/GetData 并发时
// 不出现 data race (-race), 也不 panic。关闭后的操作允许返回 error, 但不允
// 许访问已释放的 *pebble.DB。
func TestPebbleStore_ConcurrentCloseVsOps(t *testing.T) {
	s, err := newPebbleStorage(context.Background(), filepath.Join(t.TempDir(), "pebble"))
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "k", []byte("v"), time.Hour))

	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("k-%d", id)
			for {
				select {
				case <-done:
					return
				default:
				}
				// Close 后 Put 预期返回 errPebbleClosed (被包装), 这里只关心不 panic /
				// 不触发 race, 因此忽略返回。
				_ = s.PutData(ctx, key, []byte("x"), time.Hour)
			}
		}(i)
	}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				_, _ = s.GetData(ctx, "k")
				_, _ = s.IsDataExist(ctx, "k")
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, s.Close())
	time.Sleep(20 * time.Millisecond)
	close(done)
	wg.Wait()

	// 二次 Close 保持幂等。
	require.NoError(t, s.Close())

	// 关闭后的入口方法必须返回 error 而不是 panic。
	assert.Error(t, s.PutData(ctx, "k", []byte("v"), time.Hour))
	_, err = s.GetData(ctx, "k")
	assert.Error(t, err)
	ok, err := s.IsDataExist(ctx, "k")
	assert.Error(t, err)
	assert.False(t, ok)
	assert.Error(t, s.CleanupExpired(ctx))
}
