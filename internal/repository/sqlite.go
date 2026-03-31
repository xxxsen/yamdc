package repository

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/glebarez/go-sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type SQLite struct {
	db *sql.DB
}

func NewSQLite(path string) (*SQLite, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	repo := &SQLite{db: db}
	if err := repo.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (s *SQLite) DB() *sql.DB {
	return s.db
}

func (s *SQLite) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLite) init(ctx context.Context) error {
	if err := applyMigrations(ctx, s.db); err != nil {
		return err
	}
	return ensureJobConflictSchema(ctx, s.db)
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

func ensureJobConflictSchema(ctx context.Context, db *sql.DB) error {
	hasColumn, err := hasColumnNamed(ctx, db, "yamdc_job_tab", "conflict_key")
	if err != nil {
		return err
	}
	if !hasColumn {
		if _, err := db.ExecContext(ctx, `ALTER TABLE yamdc_job_tab ADD COLUMN conflict_key TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add yamdc_job_tab.conflict_key failed: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_yamdc_job_conflict_key_active ON yamdc_job_tab(conflict_key) WHERE deleted_at = 0 AND status != 'done' AND conflict_key != ''`); err != nil {
		return fmt.Errorf("create yamdc_job_tab conflict index failed: %w", err)
	}
	return nil
}

func hasColumnNamed(ctx context.Context, db *sql.DB, tableName string, columnName string) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+tableName+`)`)
	if err != nil {
		return false, fmt.Errorf("read table info for %s failed: %w", tableName, err)
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan table info for %s failed: %w", tableName, err)
		}
		if name == columnName {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table info for %s failed: %w", tableName, err)
	}
	return false, nil
}
