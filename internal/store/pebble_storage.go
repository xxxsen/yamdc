package store

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
)

const (
	pebbleDataPrefix   byte = 'd'
	pebbleExpirePrefix byte = 'e'

	pebbleRecordHeaderSize = 8
	pebbleCleanupBatchSize = 1024
)

var (
	errPebbleRecordTooShort = errors.New("pebble cache record too short")
	errPebbleBadExpireKey   = errors.New("bad pebble cache expire key")
	errCacheExpired         = errors.New("cache entry expired")
	errPebbleClosed         = errors.New("pebble cache db closed")
)

type pebbleStore struct {
	db *pebble.DB
}

func normalizeExpire(expire time.Duration) time.Duration {
	if expire == 0 {
		return defaultExpireTime
	}
	if expire < 0 {
		return time.Nanosecond
	}
	return expire
}

func makePebbleDataKey(key string) []byte {
	out := make([]byte, 1+len(key))
	out[0] = pebbleDataPrefix
	copy(out[1:], key)
	return out
}

func makePebbleExpireKey(expireAt int64, key string) []byte {
	out := make([]byte, 1+pebbleRecordHeaderSize+len(key))
	out[0] = pebbleExpirePrefix
	binary.BigEndian.PutUint64(out[1:1+pebbleRecordHeaderSize], uint64(expireAt))
	copy(out[1+pebbleRecordHeaderSize:], key)
	return out
}

func pebbleExpireLowerBound() []byte {
	return []byte{pebbleExpirePrefix}
}

func pebbleExpireUpperBound() []byte {
	return []byte{pebbleExpirePrefix + 1}
}

func parsePebbleExpireKey(raw []byte) (int64, string, error) {
	if len(raw) < 1+pebbleRecordHeaderSize || raw[0] != pebbleExpirePrefix {
		return 0, "", errPebbleBadExpireKey
	}
	expireAt := int64(binary.BigEndian.Uint64(raw[1 : 1+pebbleRecordHeaderSize]))
	key := string(raw[1+pebbleRecordHeaderSize:])
	return expireAt, key, nil
}

func encodePebbleRecord(expireAt int64, value []byte) []byte {
	out := make([]byte, pebbleRecordHeaderSize+len(value))
	binary.BigEndian.PutUint64(out[:pebbleRecordHeaderSize], uint64(expireAt))
	copy(out[pebbleRecordHeaderSize:], value)
	return out
}

func decodePebbleRecord(raw []byte) (int64, []byte, error) {
	if len(raw) < pebbleRecordHeaderSize {
		return 0, nil, errPebbleRecordTooShort
	}
	expireAt := int64(binary.BigEndian.Uint64(raw[:pebbleRecordHeaderSize]))
	return expireAt, raw[pebbleRecordHeaderSize:], nil
}

func NewPebbleStorage(ctx context.Context, path string) (IStorage, error) {
	return newPebbleStorage(ctx, path)
}

func newPebbleStorage(ctx context.Context, path string) (*pebbleStore, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create pebble cache dir %s failed: %w", path, err)
	}
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("open pebble cache db %s failed: %w", path, err)
	}
	s := &pebbleStore{db: db}
	if err := s.CleanupExpired(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func MustNewPebbleStorage(ctx context.Context, path string) IStorage {
	s, err := NewPebbleStorage(ctx, path)
	if err != nil {
		panic(err)
	}
	return s
}

func (s *pebbleStore) PutData(ctx context.Context, key string, value []byte, expire time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	db, err := s.openDB()
	if err != nil {
		return err
	}
	expireAt := time.Now().Add(normalizeExpire(expire)).UnixNano()
	dataKey := makePebbleDataKey(key)
	expireKey := makePebbleExpireKey(expireAt, key)

	batch := db.NewBatch()
	defer func() {
		_ = batch.Close()
	}()

	oldExpireAt, ok, err := s.lookupExpireAt(dataKey)
	if err != nil {
		return fmt.Errorf("lookup old cache key %s failed: %w", key, err)
	}
	if ok {
		if err := batch.Delete(makePebbleExpireKey(oldExpireAt, key), pebble.NoSync); err != nil {
			return fmt.Errorf("delete old cache expire index %s failed: %w", key, err)
		}
	}
	if err := batch.Set(dataKey, encodePebbleRecord(expireAt, value), pebble.NoSync); err != nil {
		return fmt.Errorf("put cache key %s failed: %w", key, err)
	}
	if err := batch.Set(expireKey, nil, pebble.NoSync); err != nil {
		return fmt.Errorf("put cache expire index %s failed: %w", key, err)
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("commit cache key %s failed: %w", key, err)
	}
	return nil
}

func (s *pebbleStore) lookupExpireAt(dataKey []byte) (int64, bool, error) {
	db, err := s.openDB()
	if err != nil {
		return 0, false, err
	}
	raw, closer, err := db.Get(dataKey)
	if errors.Is(err, pebble.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	defer func() {
		_ = closer.Close()
	}()
	expireAt, _, err := decodePebbleRecord(raw)
	if err != nil {
		return 0, false, err
	}
	return expireAt, true, nil
}

func (s *pebbleStore) GetData(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, expireAt, err := s.getRecord(key)
	if err != nil {
		return nil, fmt.Errorf("query cache key %s failed: %w", key, err)
	}
	if expireAt <= time.Now().UnixNano() {
		_ = s.deleteExpiredKey(key, expireAt)
		return nil, fmt.Errorf("query cache key %s failed: %w", key, errCacheExpired)
	}
	return value, nil
}

func (s *pebbleStore) getRecord(key string) ([]byte, int64, error) {
	db, openErr := s.openDB()
	if openErr != nil {
		return nil, 0, openErr
	}
	raw, closer, err := db.Get(makePebbleDataKey(key))
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = closer.Close()
	}()
	expireAt, value, err := decodePebbleRecord(raw)
	if err != nil {
		return nil, 0, err
	}
	return append([]byte(nil), value...), expireAt, nil
}

func (s *pebbleStore) IsDataExist(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	_, expireAt, err := s.getRecord(key)
	if errors.Is(err, pebble.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check cache key %s existence failed: %w", key, err)
	}
	if expireAt <= time.Now().UnixNano() {
		_ = s.deleteExpiredKey(key, expireAt)
		return false, nil
	}
	return true, nil
}

func (s *pebbleStore) deleteExpiredKey(key string, expireAt int64) error {
	currentExpireAt, ok, err := s.lookupExpireAt(makePebbleDataKey(key))
	if err != nil {
		return err
	}
	db, err := s.openDB()
	if err != nil {
		return err
	}
	batch := db.NewBatch()
	defer func() {
		_ = batch.Close()
	}()
	if ok && currentExpireAt == expireAt {
		if err := batch.Delete(makePebbleDataKey(key), pebble.NoSync); err != nil {
			return err
		}
	}
	if err := batch.Delete(makePebbleExpireKey(expireAt, key), pebble.NoSync); err != nil {
		return err
	}
	return batch.Commit(pebble.Sync)
}

func (s *pebbleStore) CleanupExpired(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	db, err := s.openDB()
	if err != nil {
		return err
	}
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: pebbleExpireLowerBound(),
		UpperBound: pebbleExpireUpperBound(),
	})
	if err != nil {
		return fmt.Errorf("create pebble cache cleanup iterator failed: %w", err)
	}
	defer func() {
		_ = iter.Close()
	}()

	now := time.Now().UnixNano()
	batch := db.NewBatch()
	defer func() {
		_ = batch.Close()
	}()
	pending := 0
	for valid := iter.First(); valid; valid = iter.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		expireKey := append([]byte(nil), iter.Key()...)
		expireAt, key, err := parsePebbleExpireKey(expireKey)
		if err != nil {
			continue
		}
		if expireAt > now {
			break
		}
		currentExpireAt, ok, err := s.lookupExpireAt(makePebbleDataKey(key))
		if err != nil && !errors.Is(err, pebble.ErrNotFound) {
			return fmt.Errorf("lookup cache key %s during cleanup failed: %w", key, err)
		}
		if ok && currentExpireAt == expireAt {
			if err := batch.Delete(makePebbleDataKey(key), pebble.NoSync); err != nil {
				return fmt.Errorf("delete cache key %s during cleanup failed: %w", key, err)
			}
		}
		if err := batch.Delete(expireKey, pebble.NoSync); err != nil {
			return fmt.Errorf("delete cache expire index during cleanup failed: %w", err)
		}
		pending++
		if pending >= pebbleCleanupBatchSize {
			if err := batch.Commit(pebble.Sync); err != nil {
				return fmt.Errorf("commit cache cleanup batch failed: %w", err)
			}
			if err := batch.Close(); err != nil {
				return fmt.Errorf("close cache cleanup batch failed: %w", err)
			}
			batch = db.NewBatch()
			pending = 0
		}
	}
	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterate cache expire index failed: %w", err)
	}
	if pending == 0 {
		return nil
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("commit cache cleanup batch failed: %w", err)
	}
	return nil
}

func (s *pebbleStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	db := s.db
	s.db = nil
	if err := db.Close(); err != nil {
		return fmt.Errorf("close pebble cache db failed: %w", err)
	}
	return nil
}

func (s *pebbleStore) openDB() (*pebble.DB, error) {
	if s == nil || s.db == nil {
		return nil, errPebbleClosed
	}
	return s.db, nil
}

func PebblePathForDataDir(dataDir string) string {
	return filepath.Join(dataDir, "cache", "pebble")
}
