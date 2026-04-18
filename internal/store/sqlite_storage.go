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
	defaultExpireTime = 90 * 24 * time.Hour // 默认存储3个月, 超过就删了吧, 实在没想到有啥东西需要永久存储的?

	// CacheCleanupInterval 是缓存过期行的清理周期, 由 internal/cronscheduler
	// 注册的 cache_store_cleanup job 负责按本周期触发一次 CleanupExpired。
	//
	// 24 小时是保守值: 缓存失效是 "到期后读不到即可", 不要求立即物理删除;
	// 过频清理会抢读路径的 sqlite 锁, 过疏清理会让过期数据占盘更久。DB 规模
	// 不大 (搜索/评分相关元数据), 过期行延迟到次日再扫一次成本可忽略。
	// 如果将来缓存规模上来了, 再考虑拆成 "过期时懒删" + "夜间批量清理" 两级。
	CacheCleanupInterval = 24 * time.Hour
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
	if err := s.CleanupExpired(ctx); err != nil {
		return err
	}
	return nil
}

// CleanupExpired 删掉所有 expire_at <= now 的缓存行。
//
// 导出而非私有: cronscheduler 的 cache_store_cleanup job 需要从外部调本方法
// 触发周期清理 (见 internal/cronscheduler + store.NewCacheCleanupJob)。
// 同时 init 里也会顺手跑一次, 相当于进程重启时把历史过期行清掉, 避免等到
// 下一轮 cron tick 才清理。
func (s *sqliteStore) CleanupExpired(ctx context.Context) error {
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

func newSqliteStorage(ctx context.Context, path string) (*sqliteStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir %s failed: %w", dir, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %s failed: %w", path, err)
	}
	configureSqliteStoreDB(ctx, db)
	s := &sqliteStore{db: db}
	if err := s.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func NewSqliteStorage(ctx context.Context, path string) (IStorage, error) {
	return newSqliteStorage(ctx, path)
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
	_, _ = db.ExecContext(ctx, `PRAGMA busy_timeout = 10000`)
}

func (s *sqliteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close sqlite db failed: %w", err)
	}
	return nil
}
