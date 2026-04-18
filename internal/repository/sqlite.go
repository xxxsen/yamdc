package repository

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
	"strconv"
	"strings"

	_ "github.com/glebarez/go-sqlite" // register sqlite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// err113 要求 migration 层暴露的错误是静态 sentinel, 以便调用方做 errors.Is 断言。
// 这几个主要在启动期 bubble up, 实际没有回复路径, 但 lint 规则统一要求。
var (
	errMigrationDuplicateVersion = errors.New("duplicate migration version")
	errMigrationBadName          = errors.New("migration file has invalid name")
	errMigrationBadVersion       = errors.New("migration file has invalid version")
)

type SQLite struct {
	db *sql.DB
}

func NewSQLite(ctx context.Context, path string) (*SQLite, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir %s failed: %w", dir, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %s failed: %w", path, err)
	}
	configureSQLite(ctx, db)
	repo := &SQLite{db: db}
	if err := repo.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func configureSQLite(ctx context.Context, db *sql.DB) {
	if db == nil {
		return
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	_, _ = db.ExecContext(ctx, `PRAGMA busy_timeout = 5000`)
}

func (s *SQLite) DB() *sql.DB {
	return s.db
}

func (s *SQLite) Close() error {
	if s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close sqlite db failed: %w", err)
	}
	return nil
}

func (s *SQLite) init(ctx context.Context) error {
	return applyMigrations(ctx, s.db)
}

// applyMigrations 按 "NNN_<name>.sql" 里的 NNN 序号顺序执行 migrations/ 下的
// 文件, 并通过 PRAGMA user_version 记录已应用的最高序号, 跳过已执行过的文件。
//
// 为什么不直接每次 "replay 全部": 早期 migrations (001_init) 里全部语句都带
// `IF NOT EXISTS`, 重放幂等, 但后续 migration 难免要用 `ALTER TABLE ADD COLUMN`
// 这类非幂等语句, 第二次 open 就会失败。PRAGMA user_version 是 SQLite 内置
// 的一个 int32 header 字段, 不需要额外建表, 刚好够用来做轻量版本化。
//
// 这是 3.4 / 1.4 推进过程中按需引入的最小实现, roadmap 2.1 要把完整的
// migration 版本化做成一个独立任务, 届时可以替换为更正式的 schema_migrations
// 表 (记录 checksum / 应用时间 / 失败重试语义等)。
func applyMigrations(ctx context.Context, db *sql.DB) error {
	currentVersion, err := readUserVersion(ctx, db)
	if err != nil {
		return err
	}
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}
		content, err := fs.ReadFile(migrationsFS, "migrations/"+m.file)
		if err != nil {
			return fmt.Errorf("read migration %s failed: %w", m.file, err)
		}
		for _, q := range splitSQLStatements(string(content)) {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("execute migration %s failed: %w", m.file, err)
			}
		}
		// PRAGMA user_version 不支持占位符绑定, 只能走字符串拼接, 因此
		// 版本号来源 (迁移文件名前缀) 必须在 loadMigrations 里先用 strconv 解析过,
		// 这里拿到的已经是可信整数, 没有注入风险。
		if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
			return fmt.Errorf("bump user_version after %s failed: %w", m.file, err)
		}
	}
	return nil
}

type migrationFile struct {
	version int
	file    string
}

// splitSQLStatements 把一整段 SQL 文本切成多个独立语句。
//
// 为什么不直接 strings.Split(content, ";"):
//
//	早期实现是这么做的, 但 "--" 行注释里的中文标点 (分号/句号) 会把
//	一条语义完整的 SQL 切断, 下游 Exec 只会看到半句, 报 "syntax error",
//	且错误位置指向中文字里, 非常难排查。
//	历史上 002 / 003 都因为注释里写了分号栽过同一个坑。
//
// 这里先按行剥掉 "--" 行注释 (SQL 的行注释不跨行), 再按 ";" 切。
// 我们的 migration 只定义 schema, 不出现字符串字面量里含 "--" 的情况,
// 所以这个简化实现是安全的; 如果将来引入 INSERT 语句带复杂字符串,
// 再考虑上 token-level 切分器。
func splitSQLStatements(content string) []string {
	var buf strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	result := make([]string, 0)
	for _, part := range strings.Split(buf.String(), ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func readUserVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read sqlite user_version failed: %w", err)
	}
	return version, nil
}

func loadMigrations() ([]migrationFile, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations failed: %w", err)
	}
	migrations := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		name := entry.Name()
		version, err := parseMigrationVersion(name)
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, migrationFile{version: version, file: name})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	for i := 1; i < len(migrations); i++ {
		if migrations[i].version == migrations[i-1].version {
			return nil, fmt.Errorf("%w %d: %s and %s",
				errMigrationDuplicateVersion, migrations[i].version,
				migrations[i-1].file, migrations[i].file)
		}
	}
	return migrations, nil
}

// parseMigrationVersion 解析 "NNN_<name>.sql" 前缀的整数版本号。
// 约束: 必须以至少一位数字开头 + 下划线。不符合就拒绝, 避免后续 user_version
// 对不上号 (例如有人放了个 readme.sql 进来)。
func parseMigrationVersion(name string) (int, error) {
	idx := strings.IndexByte(name, '_')
	if idx <= 0 {
		return 0, fmt.Errorf("%w: %s (missing leading NNN_)", errMigrationBadName, name)
	}
	version, err := strconv.Atoi(name[:idx])
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %s", errMigrationBadName, name, err.Error())
	}
	if version <= 0 {
		return 0, fmt.Errorf("%w: %s: got %d (must be positive)", errMigrationBadVersion, name, version)
	}
	return version, nil
}
