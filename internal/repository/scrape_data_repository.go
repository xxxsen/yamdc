package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ScrapeData struct {
	ID         int64  `json:"id"`
	JobID      int64  `json:"job_id"`
	Source     string `json:"source"`
	Version    int64  `json:"version"`
	RawData    string `json:"raw_data"`
	ReviewData string `json:"review_data"`
	FinalData  string `json:"final_data"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

// ErrScrapeDataNotFound is returned when scrape data is not found for a job.
var ErrScrapeDataNotFound = errors.New("scrape data not found")

type ScrapeDataRepository struct {
	db *sql.DB
}

func NewScrapeDataRepository(db *sql.DB) *ScrapeDataRepository {
	return &ScrapeDataRepository{db: db}
}

func (r *ScrapeDataRepository) UpsertRawData(ctx context.Context, jobID int64, source, rawData string) error {
	now := time.Now().UnixMilli()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO yamdc_scrape_data_tab (job_id, source, version, raw_data, review_data, final_data, status, created_at,
			updated_at)
		VALUES (?, ?, 1, ?, '', '', 'draft', ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET
			source = excluded.source,
			version = yamdc_scrape_data_tab.version + 1,
			raw_data = excluded.raw_data,
			updated_at = excluded.updated_at
	`, jobID, source, rawData, now, now)
	if err != nil {
		return fmt.Errorf("upsert raw scrape data failed: %w", err)
	}
	return nil
}

func (r *ScrapeDataRepository) GetByJobID(ctx context.Context, jobID int64) (*ScrapeData, error) {
	var item ScrapeData
	err := r.db.QueryRowContext(ctx, `
		SELECT id, job_id, source, version, raw_data, review_data, final_data, status, created_at, updated_at
		FROM yamdc_scrape_data_tab
		WHERE job_id = ?
	`, jobID).Scan(
		&item.ID,
		&item.JobID,
		&item.Source,
		&item.Version,
		&item.RawData,
		&item.ReviewData,
		&item.FinalData,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrScrapeDataNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get scrape data failed: %w", err)
	}
	return &item, nil
}

func (r *ScrapeDataRepository) SaveReviewData(ctx context.Context, jobID int64, reviewData string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE yamdc_scrape_data_tab
		SET review_data = ?, status = 'reviewed', updated_at = ?
		WHERE job_id = ?
	`, reviewData, time.Now().UnixMilli(), jobID)
	if err != nil {
		return fmt.Errorf("save review data failed: %w", err)
	}
	return nil
}

func (r *ScrapeDataRepository) SaveFinalData(ctx context.Context, jobID int64, finalData string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE yamdc_scrape_data_tab
		SET final_data = ?, status = 'imported', updated_at = ?
		WHERE job_id = ?
	`, finalData, time.Now().UnixMilli(), jobID)
	if err != nil {
		return fmt.Errorf("save final data failed: %w", err)
	}
	return nil
}

func (r *ScrapeDataRepository) DeleteByJobID(ctx context.Context, jobID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM yamdc_scrape_data_tab WHERE job_id = ?`, jobID)
	if err != nil {
		return fmt.Errorf("delete scrape data failed: %w", err)
	}
	return nil
}
