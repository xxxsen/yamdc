package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xxxsen/yamdc/internal/jobdef"
)

type UpsertJobInput struct {
	FileName string
	FileExt  string
	RelPath  string
	AbsPath  string
	Number   string
	FileSize int64
}

type JobRepository struct {
	db *sql.DB
}

type ListJobsResult struct {
	Items    []jobdef.Job `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) UpsertScannedJob(ctx context.Context, in UpsertJobInput) error {
	now := time.Now().UnixMilli()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO yamdc_job_tab (
			job_uid, file_name, file_ext, rel_path, abs_path, number, file_size, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rel_path) DO UPDATE SET
			file_name = excluded.file_name,
			file_ext = excluded.file_ext,
			abs_path = excluded.abs_path,
			number = excluded.number,
			file_size = excluded.file_size,
			deleted_at = 0,
			updated_at = excluded.updated_at
	`, uuid.NewString(), in.FileName, in.FileExt, in.RelPath, in.AbsPath, in.Number, in.FileSize, jobdef.StatusInit, now, now)
	if err != nil {
		return fmt.Errorf("upsert scanned job failed: %w", err)
	}
	return nil
}

func (r *JobRepository) ListJobs(ctx context.Context, status []jobdef.Status, keyword string, page int, pageSize int) (*ListJobsResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	where := ` WHERE deleted_at = 0`
	args := make([]interface{}, 0, len(status)+4)
	if len(status) > 0 {
		where += " AND status IN ("
		for i, item := range status {
			if i > 0 {
				where += ","
			}
			where += "?"
			args = append(args, item)
		}
		where += ")"
	}
	if keyword != "" {
		where += " AND (file_name LIKE ? OR rel_path LIKE ? OR number LIKE ?)"
		like := "%" + keyword + "%"
		args = append(args, like, like, like)
	}
	var total int
	countQuery := `SELECT count(*) FROM yamdc_job_tab` + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count jobs failed: %w", err)
	}
	query := `
		SELECT id, job_uid, file_name, file_ext, rel_path, abs_path, number, file_size, status, error_msg, created_at, updated_at
		FROM yamdc_job_tab
	` + where + ` ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list jobs failed: %w", err)
	}
	defer rows.Close()

	jobs := make([]jobdef.Job, 0, max(pageSize, 16))
	for rows.Next() {
		var item jobdef.Job
		if err := rows.Scan(
			&item.ID,
			&item.JobUID,
			&item.FileName,
			&item.FileExt,
			&item.RelPath,
			&item.AbsPath,
			&item.Number,
			&item.FileSize,
			&item.Status,
			&item.ErrorMsg,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job failed: %w", err)
		}
		jobs = append(jobs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs failed: %w", err)
	}
	return &ListJobsResult{
		Items:    jobs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (r *JobRepository) GetByID(ctx context.Context, id int64) (*jobdef.Job, error) {
	var item jobdef.Job
	err := r.db.QueryRowContext(ctx, `
		SELECT id, job_uid, file_name, file_ext, rel_path, abs_path, number, file_size, status, error_msg, created_at, updated_at
		FROM yamdc_job_tab
		WHERE id = ? AND deleted_at = 0
	`, id).Scan(
		&item.ID,
		&item.JobUID,
		&item.FileName,
		&item.FileExt,
		&item.RelPath,
		&item.AbsPath,
		&item.Number,
		&item.FileSize,
		&item.Status,
		&item.ErrorMsg,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get job failed: %w", err)
	}
	return &item, nil
}

func (r *JobRepository) UpdateStatus(ctx context.Context, id int64, from []jobdef.Status, to jobdef.Status, errMsg string) (bool, error) {
	query := `UPDATE yamdc_job_tab SET status = ?, error_msg = ?, updated_at = ? WHERE id = ?`
	args := make([]interface{}, 0, len(from)+4)
	args = append(args, to, errMsg, time.Now().UnixMilli(), id)
	if len(from) > 0 {
		query += " AND status IN ("
		for i, item := range from {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, item)
		}
		query += ")"
	}
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("update job status failed: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read rows affected failed: %w", err)
	}
	return affected > 0, nil
}

func (r *JobRepository) MarkDone(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE yamdc_job_tab
		SET status = ?, error_msg = '', imported_at = ?, updated_at = ?
		WHERE id = ?
	`, jobdef.StatusDone, time.Now().UnixMilli(), time.Now().UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("mark job done failed: %w", err)
	}
	return nil
}

func (r *JobRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE yamdc_job_tab
		SET deleted_at = ?, updated_at = ?
		WHERE id = ?
	`, time.Now().UnixMilli(), time.Now().UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("soft delete job failed: %w", err)
	}
	return nil
}

func (r *JobRepository) RecoverProcessingJobs(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE yamdc_job_tab
		SET status = ?, error_msg = 'server restarted while processing', updated_at = ?
		WHERE status = ? AND deleted_at = 0
	`, jobdef.StatusFailed, time.Now().UnixMilli(), jobdef.StatusProcessing)
	if err != nil {
		return fmt.Errorf("recover processing jobs failed: %w", err)
	}
	return nil
}
