package store

import (
	"bytes"
	"context"
	"errors"
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

	require.NoError(t, s.PutData(ctx, "expired", []byte("value"), time.Millisecond))
	assert.Eventually(t, func() bool {
		return s.CleanupExpired(ctx) == nil && countPebbleKeys(t, s, []byte{pebbleExpirePrefix}) == 0
	}, 5*time.Second, 20*time.Millisecond)

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
	require.NoError(t, s.PutData(ctx, "k", []byte("value"), time.Millisecond))

	assert.Eventually(t, func() bool {
		_, err := s.GetData(ctx, "k")
		return err != nil
	}, 5*time.Second, 20*time.Millisecond)
}

func TestPebbleStore_ExpireExist(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	require.NoError(t, s.PutData(ctx, "k", []byte("value"), time.Millisecond))

	assert.Eventually(t, func() bool {
		ok, err := s.IsDataExist(ctx, "k")
		return err == nil && !ok
	}, 5*time.Second, 20*time.Millisecond)
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
	require.NoError(t, s.PutData(ctx, "expired", []byte("old"), time.Millisecond))
	require.NoError(t, s.PutData(ctx, "alive", []byte("new"), time.Hour))

	assert.Eventually(t, func() bool {
		return s.CleanupExpired(ctx) == nil && countPebbleKeys(t, s, []byte{pebbleExpirePrefix}) == 1
	}, 5*time.Second, 20*time.Millisecond)
	got, err := s.GetData(ctx, "alive")
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), got)
}

func TestPebbleStore_CleanupSkipsNewerValueWithStaleIndex(t *testing.T) {
	s := newTestPebbleStore(t)
	ctx := context.Background()
	staleExpireAt := time.Now().Add(-time.Hour).UnixNano()
	freshExpireAt := time.Now().Add(time.Hour).UnixNano()
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
