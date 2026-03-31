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
	return applyMigrations(ctx, s.db)
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
