package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

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
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS yamdc_job_tab (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_uid TEXT NOT NULL UNIQUE,
			file_name TEXT NOT NULL,
			file_ext TEXT NOT NULL,
			rel_path TEXT NOT NULL UNIQUE,
			abs_path TEXT NOT NULL,
			number TEXT NOT NULL,
			file_size INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			error_msg TEXT NOT NULL DEFAULT '',
			retry_count INTEGER NOT NULL DEFAULT 0,
			scrape_started_at INTEGER NOT NULL DEFAULT 0,
			scrape_finished_at INTEGER NOT NULL DEFAULT 0,
			reviewed_at INTEGER NOT NULL DEFAULT 0,
			imported_at INTEGER NOT NULL DEFAULT 0,
			deleted_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_yamdc_job_status ON yamdc_job_tab(status);`,
		`CREATE INDEX IF NOT EXISTS idx_yamdc_job_updated_at ON yamdc_job_tab(updated_at);`,
		`CREATE TABLE IF NOT EXISTS yamdc_log_tab (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL,
			level TEXT NOT NULL,
			stage TEXT NOT NULL,
			message TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_yamdc_log_job_id_created_at ON yamdc_log_tab(job_id, created_at);`,
		`CREATE TABLE IF NOT EXISTS yamdc_scrape_data_tab (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL UNIQUE,
			source TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			raw_data TEXT NOT NULL DEFAULT '',
			review_data TEXT NOT NULL DEFAULT '',
			final_data TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec init stmt failed: %w", err)
		}
	}
	return nil
}
