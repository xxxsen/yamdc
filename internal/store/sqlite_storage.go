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

	_ "github.com/glebarez/go-sqlite"
)

const (
	defaultExpireTime = 90 * 24 * time.Hour //默认存储3个月, 超过就删了吧, 实在没想到有啥东西需要永久存储的?
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type sqliteStore struct {
	db *sql.DB
}

func (s *sqliteStore) init(ctx context.Context) error {
	if err := applyMigrations(ctx, s.db); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM cache_tab WHERE expire_at <= ?", time.Now().Unix()); err != nil {
		return fmt.Errorf("cleanup expired cache failed: %w", err)
	}
	return nil
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
		return nil, err
	}
	return val, nil
}

func (s *sqliteStore) PutData(ctx context.Context, key string, value []byte, expire time.Duration) error {
	var expireAt int64 = 0
	if expire == 0 {
		expire = defaultExpireTime //use default expire time
	}
	if expire > 0 {
		expireAt = time.Now().Add(expire).Unix()
	}
	_, err := s.db.ExecContext(ctx, "INSERT OR REPLACE INTO cache_tab (key, value, expire_at) VALUES (?, ?, ?)", key, value, expireAt)
	if err != nil {
		return err
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
		return false, err
	}
	if cnt == 0 {
		return false, nil
	}
	return true, nil
}

func NewSqliteStorage(path string) (IStorage, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &sqliteStore{db: db}
	if err := s.init(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func MustNewSqliteStorage(path string) IStorage {
	s, err := NewSqliteStorage(path)
	if err != nil {
		panic(err)
	}
	return s
}
