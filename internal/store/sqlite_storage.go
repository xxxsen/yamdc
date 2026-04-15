package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite" // register sqlite driver
)

const (
	defaultExpireTime      = 90 * 24 * time.Hour // 默认存储3个月, 超过就删了吧, 实在没想到有啥东西需要永久存储的?
	defaultCleanupInterval = 24 * time.Hour
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type sqliteStore struct {
	db              *sql.DB
	cleanupInterval time.Duration
	cleanupCancel   context.CancelFunc
}

func (s *sqliteStore) init(ctx context.Context) error {
	if err := applyMigrations(ctx, s.db); err != nil {
		return err
	}
	if err := s.cleanupExpired(ctx); err != nil {
		return err
	}
	return nil
}

func (s *sqliteStore) cleanupExpired(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM cache_tab WHERE expire_at <= ?", time.Now().Unix()); err != nil {
		return fmt.Errorf("cleanup expired cache failed: %w", err)
	}
	return nil
}

func (s *sqliteStore) startCleanupLoop(ctx context.Context) {
	if s.cleanupInterval <= 0 {
		return
	}
	loopCtx, cancel := context.WithCancel(ctx)
	s.cleanupCancel = cancel
	go func() { //nolint:gosec // background cleanup goroutine
		ticker := time.NewTicker(s.cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-loopCtx.Done():
				return
			case <-ticker.C:
				_ = s.cleanupExpired(context.Background()) //nolint:contextcheck // background cleanup goroutine
			}
		}
	}()
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations failed: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for _, file := range files {
		content, err := fs.ReadFile(migrationsFS, "migrations/"+file)
		if err != nil {
			return fmt.Errorf("read migration %s failed: %w", file, err)
		}
		queries := strings.Split(string(content), ";")
		for _, q := range queries {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("execute migration %s failed: %w", file, err)
			}
		}
	}
	return nil
}

func (s *sqliteStore) GetData(ctx context.Context, key string) ([]byte, error) {
	var val []byte
	now := time.Now().Unix()
	err := s.db.QueryRowContext(ctx, "SELECT value FROM cache_tab WHERE key = ? and expire_at > ?", key, now).Scan(&val)
	if err != nil {
		return nil, fmt.Errorf("query cache key %s failed: %w", key, err)
	}
	return val, nil
}

func (s *sqliteStore) PutData(ctx context.Context, key string, value []byte, expire time.Duration) error {
	var expireAt int64
	if expire == 0 {
		expire = defaultExpireTime // use default expire time
	}
	if expire > 0 {
		expireAt = time.Now().Add(expire).Unix()
	}
	_, err := s.db.ExecContext(
		ctx,
		"INSERT OR REPLACE INTO cache_tab (key, value, expire_at) VALUES (?, ?, ?)",
		key,
		value,
		expireAt,
	)
	if err != nil {
		return fmt.Errorf("put cache key %s failed: %w", key, err)
	}
	return nil
}

func (s *sqliteStore) IsDataExist(ctx context.Context, key string) (bool, error) {
	var cnt int64
	now := time.Now().Unix()
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM cache_tab WHERE key = ? and expire_at > ?", key, now).Scan(&cnt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check cache key %s existence failed: %w", key, err)
	}
	if cnt == 0 {
		return false, nil
	}
	return true, nil
}

func newSqliteStorage(ctx context.Context, path string, cleanupInterval time.Duration) (*sqliteStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir %s failed: %w", dir, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %s failed: %w", path, err)
	}
	configureSqliteStoreDB(ctx, db)
	s := &sqliteStore{
		db:              db,
		cleanupInterval: cleanupInterval,
	}
	if err := s.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	s.startCleanupLoop(ctx)
	return s, nil
}

func NewSqliteStorage(ctx context.Context, path string) (IStorage, error) {
	return newSqliteStorage(ctx, path, defaultCleanupInterval)
}

func MustNewSqliteStorage(ctx context.Context, path string) IStorage {
	s, err := NewSqliteStorage(ctx, path)
	if err != nil {
		panic(err)
	}
	return s
}

func configureSqliteStoreDB(ctx context.Context, db *sql.DB) {
	if db == nil {
		return
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	_, _ = db.ExecContext(ctx, `PRAGMA busy_timeout = 5000`)
}

func (s *sqliteStore) Close() error {
	if s == nil {
		return nil
	}
	if s.cleanupCancel != nil {
		s.cleanupCancel()
		s.cleanupCancel = nil
	}
	if s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close sqlite db failed: %w", err)
	}
	return nil
}
