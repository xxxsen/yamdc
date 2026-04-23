package store

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
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
	errPebbleExpireOverflow = errors.New("pebble cache expire timestamp overflows int64")
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
	putPebbleExpireAt(out[1:1+pebbleRecordHeaderSize], expireAt)
	copy(out[1+pebbleRecordHeaderSize:], key)
	return out
}

func putPebbleExpireAt(dst []byte, expireAt int64) {
	if expireAt < 0 {
		expireAt = 0
	}
	binary.BigEndian.PutUint64(dst, uint64(expireAt))
}

func readPebbleExpireAt(raw []byte) (int64, error) {
	expireAt := binary.BigEndian.Uint64(raw)
	if expireAt > math.MaxInt64 {
		return 0, errPebbleExpireOverflow
	}
	return int64(expireAt), nil
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
	expireAt, err := readPebbleExpireAt(raw[1 : 1+pebbleRecordHeaderSize])
	if err != nil {
		return 0, "", err
	}
	key := string(raw[1+pebbleRecordHeaderSize:])
	return expireAt, key, nil
}

func encodePebbleRecord(expireAt int64, value []byte) []byte {
	out := make([]byte, pebbleRecordHeaderSize+len(value))
	putPebbleExpireAt(out[:pebbleRecordHeaderSize], expireAt)
	copy(out[pebbleRecordHeaderSize:], value)
	return out
}

func decodePebbleRecord(raw []byte) (int64, []byte, error) {
	if len(raw) < pebbleRecordHeaderSize {
		return 0, nil, errPebbleRecordTooShort
	}
	expireAt, err := readPebbleExpireAt(raw[:pebbleRecordHeaderSize])
	if err != nil {
		return 0, nil, err
	}
	return expireAt, raw[pebbleRecordHeaderSize:], nil
}

func NewPebbleStorage(ctx context.Context, path string) (IStorage, error) {
	return newPebbleStorage(ctx, path)
}

func newPebbleStorage(ctx context.Context, path string) (*pebbleStore, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("open pebble cache canceled: %w", err)
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
		return nil, fmt.Errorf("cleanup pebble cache during open failed: %w", err)
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
		return fmt.Errorf("put cache key %s canceled: %w", key, err)
	}
	db, err := s.openDB()
	if err != nil {
		return fmt.Errorf("open pebble cache for put key %s failed: %w", key, err)
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
		return 0, false, fmt.Errorf("open pebble cache for expiration lookup failed: %w", err)
	}
	raw, closer, err := db.Get(dataKey)
	if errors.Is(err, pebble.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get cache record expiration failed: %w", err)
	}
	defer func() {
		_ = closer.Close()
	}()
	expireAt, _, err := decodePebbleRecord(raw)
	if err != nil {
		return 0, false, fmt.Errorf("decode cache record expiration failed: %w", err)
	}
	return expireAt, true, nil
}

func (s *pebbleStore) GetData(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("query cache key %s canceled: %w", key, err)
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
		return nil, 0, fmt.Errorf("open pebble cache for get key %s failed: %w", key, openErr)
	}
	raw, closer, err := db.Get(makePebbleDataKey(key))
	if err != nil {
		return nil, 0, fmt.Errorf("get cache key %s failed: %w", key, err)
	}
	defer func() {
		_ = closer.Close()
	}()
	expireAt, value, err := decodePebbleRecord(raw)
	if err != nil {
		return nil, 0, fmt.Errorf("decode cache key %s failed: %w", key, err)
	}
	return append([]byte(nil), value...), expireAt, nil
}

func (s *pebbleStore) IsDataExist(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("check cache key %s existence canceled: %w", key, err)
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
		return fmt.Errorf("lookup cache key %s before expired delete failed: %w", key, err)
	}
	db, err := s.openDB()
	if err != nil {
		return fmt.Errorf("open pebble cache for expired delete key %s failed: %w", key, err)
	}
	batch := db.NewBatch()
	defer func() {
		_ = batch.Close()
	}()
	if ok && currentExpireAt == expireAt {
		if err := batch.Delete(makePebbleDataKey(key), pebble.NoSync); err != nil {
			return fmt.Errorf("delete expired cache key %s failed: %w", key, err)
		}
	}
	if err := batch.Delete(makePebbleExpireKey(expireAt, key), pebble.NoSync); err != nil {
		return fmt.Errorf("delete expired cache index %s failed: %w", key, err)
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("commit expired cache key %s delete failed: %w", key, err)
	}
	return nil
}

func (s *pebbleStore) CleanupExpired(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup pebble cache canceled: %w", err)
	}
	db, err := s.openDB()
	if err != nil {
		return fmt.Errorf("open pebble cache for cleanup failed: %w", err)
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

	cleaner := newPebbleExpiredCleaner(db, time.Now().UnixNano())
	defer cleaner.close()
	if err := s.cleanupExpiredIterator(ctx, iter, cleaner); err != nil {
		return err
	}
	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterate cache expire index failed: %w", err)
	}
	return cleaner.commit()
}

func (s *pebbleStore) cleanupExpiredIterator(
	ctx context.Context,
	iter *pebble.Iterator,
	cleaner *pebbleExpiredCleaner,
) error {
	for valid := iter.First(); valid; valid = iter.Next() {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("cleanup pebble cache canceled: %w", err)
		}
		expireKey := append([]byte(nil), iter.Key()...)
		expireAt, key, err := parsePebbleExpireKey(expireKey)
		if err != nil {
			continue
		}
		if expireAt > cleaner.now {
			break
		}
		if err := s.queueExpiredDeletes(cleaner, expireKey, expireAt, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *pebbleStore) queueExpiredDeletes(
	cleaner *pebbleExpiredCleaner,
	expireKey []byte,
	expireAt int64,
	key string,
) error {
	currentExpireAt, ok, err := s.lookupExpireAt(makePebbleDataKey(key))
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return fmt.Errorf("lookup cache key %s during cleanup failed: %w", key, err)
	}
	if ok && currentExpireAt == expireAt {
		if err := cleaner.delete(makePebbleDataKey(key)); err != nil {
			return fmt.Errorf("delete cache key %s during cleanup failed: %w", key, err)
		}
	}
	if err := cleaner.delete(expireKey); err != nil {
		return fmt.Errorf("delete cache expire index during cleanup failed: %w", err)
	}
	return nil
}

type pebbleExpiredCleaner struct {
	db      *pebble.DB
	batch   *pebble.Batch
	pending int
	now     int64
}

func newPebbleExpiredCleaner(db *pebble.DB, now int64) *pebbleExpiredCleaner {
	return &pebbleExpiredCleaner{
		db:    db,
		batch: db.NewBatch(),
		now:   now,
	}
}

func (c *pebbleExpiredCleaner) delete(key []byte) error {
	if err := c.batch.Delete(key, pebble.NoSync); err != nil {
		return fmt.Errorf("queue cache cleanup delete failed: %w", err)
	}
	c.pending++
	if c.pending < pebbleCleanupBatchSize {
		return nil
	}
	return c.commitAndReset()
}

func (c *pebbleExpiredCleaner) commitAndReset() error {
	if err := c.commit(); err != nil {
		return err
	}
	if err := c.batch.Close(); err != nil {
		return fmt.Errorf("close cache cleanup batch failed: %w", err)
	}
	c.batch = c.db.NewBatch()
	return nil
}

func (c *pebbleExpiredCleaner) commit() error {
	if c.pending == 0 {
		return nil
	}
	if err := c.batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("commit cache cleanup batch failed: %w", err)
	}
	c.pending = 0
	return nil
}

func (c *pebbleExpiredCleaner) close() {
	if c == nil || c.batch == nil {
		return
	}
	_ = c.batch.Close()
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
