package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type LogItem struct {
	ID        int64  `json:"id"`
	JobID     int64  `json:"job_id"`
	Level     string `json:"level"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	Detail    string `json:"detail"`
	CreatedAt int64  `json:"created_at"`
}

type LogRepository struct {
	db *sql.DB
}

func NewLogRepository(db *sql.DB) *LogRepository {
	return &LogRepository{db: db}
}

func (r *LogRepository) Add(ctx context.Context, jobID int64, level, stage, message, detail string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO yamdc_log_tab (job_id, level, stage, message, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, jobID, level, stage, message, detail, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("insert log failed: %w", err)
	}
	return nil
}

func (r *LogRepository) ListByJobID(ctx context.Context, jobID int64, limit int) ([]LogItem, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, job_id, level, stage, message, detail, created_at
		FROM yamdc_log_tab
		WHERE job_id = ?
		ORDER BY id ASC
		LIMIT ?
	`, jobID, limit)
	if err != nil {
		return nil, fmt.Errorf("list logs failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	items := make([]LogItem, 0, limit)
	for rows.Next() {
		var item LogItem
		if err := rows.Scan(
			&item.ID,
			&item.JobID,
			&item.Level,
			&item.Stage,
			&item.Message,
			&item.Detail,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan log failed: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate logs failed: %w", err)
	}
	return items, nil
}

func (r *LogRepository) DeleteByJobID(ctx context.Context, jobID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM yamdc_log_tab WHERE job_id = ?`, jobID)
	if err != nil {
		return fmt.Errorf("delete logs failed: %w", err)
	}
	return nil
}
