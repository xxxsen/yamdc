package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/xxxsen/yamdc/internal/jobdef"
)

// ErrJobNotFound is returned when a job is not found in the database.
var ErrJobNotFound = errors.New("job not found")

type UpsertJobInput struct {
	FileName              string
	FileExt               string
	RelPath               string
	AbsPath               string
	Number                string
	RawNumber             string
	CleanedNumber         string
	NumberSource          string
	NumberCleanStatus     string
	NumberCleanConfidence string
	NumberCleanWarnings   string
	FileSize              int64
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

// resolveConflictNumber 决定本次 upsert 重新计算 conflict_key 时使用哪个 number。
// 当现有 job 的 number 字段处于"冻结"状态(手动改过 / 正在 processing / 等待 review
// 用户确认)时, 继续使用 existing number 重新生成 conflict_key, 避免 scanner 重扫
// 触发规则变动时, conflict_key 与 number 脱钩。具体冻结触发条件见
// upsertScannedJobSQL 的 CASE 分支, 这里必须保持一致。
func (r *JobRepository) resolveConflictNumber(ctx context.Context, relPath, number string) (string, error) {
	var existingNumber, existingSource, existingStatus string
	err := r.db.QueryRowContext(ctx,
		`SELECT number, number_source, status FROM yamdc_job_tab WHERE rel_path = ?`, relPath,
	).Scan(&existingNumber, &existingSource, &existingStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("load existing job before upsert failed: %w", err)
	}
	if err != nil {
		// ErrNoRows: 这是首次插入, 没有"历史 number"可保留, 直接用入参即可。
		return number, nil //nolint:nilerr // sql.ErrNoRows is a not-found sentinel, not a failure
	}
	if numberFieldsFrozen(existingSource, existingStatus) {
		return existingNumber, nil
	}
	return number, nil
}

// numberFieldsFrozen 仅对应 upsertScannedJobSQL 里 canonical 三项 ——
// number / number_source / conflict_key 的 CASE 分支 (通过 numberFrozenCondition
// 展开), 并被 resolveConflictNumber 使用。cleaned_number / number_clean_* 不走
// 本函数, 见 numberFrozenCondition 注释与 SQL 中相应 CASE 的独立分支。
//
// 冻结触发条件: 手动改过番号 (number_source='manual'), 或 job 正在执行/等待
// 人工 review —— 此时 scrape_data 已有一份旧 number 的 meta 快照, 若 scanner
// 的下一次重扫把 canonical number 静默改掉, Import 落盘时 NFO 里的番号就会
// 与目录名错位, 用户不易察觉。
func numberFieldsFrozen(numberSource, status string) bool {
	if numberSource == "manual" {
		return true
	}
	switch status {
	case string(jobdef.StatusProcessing), string(jobdef.StatusReviewing):
		return true
	}
	return false
}

// numberFrozenCondition 描述"job 当前 number / number_source / conflict_key 这三
// 个核心字段此次 upsert 不可被覆盖"的 SQL 条件, 与 numberFieldsFrozen (Go 侧)
// 严格一致。触发条件:
//   - 用户手动改过番号 (number_source='manual'), 一直以来就在保护;
//   - job 正在 processing / 等待 reviewing: scrape_data 已有快照, 若 number 被
//     scanner 重扫时静默改掉, Import 会和快照错位(目录名用新 number, NFO 用旧 meta)。
//
// 注意: cleaned_number / number_clean_* 不在本条件内, 继续保留原先的 manual-only
// 覆盖语义(cleaned_number 永远刷新, number_clean_* 仅 manual 冻结), 因为它们只
// 是"cleaner 当下给出的建议值", 不参与 Import 的目录构造, 让 scanner 继续跟踪。
const numberFrozenCondition = `(` +
	`yamdc_job_tab.number_source = 'manual'` +
	` OR yamdc_job_tab.status IN ('processing','reviewing')` +
	`)`

const upsertScannedJobSQL = `` +
	`INSERT INTO yamdc_job_tab (
	job_uid, file_name, file_ext, conflict_key,
	rel_path, abs_path, number, raw_number,
	cleaned_number, number_source,
	number_clean_status, number_clean_confidence,
	number_clean_warnings, file_size, status,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(rel_path) DO UPDATE SET
	file_name = excluded.file_name,
	file_ext = excluded.file_ext,
	conflict_key = CASE
		WHEN ` + numberFrozenCondition + `
		THEN yamdc_job_tab.conflict_key
		ELSE excluded.conflict_key END,
	abs_path = excluded.abs_path,
	raw_number = excluded.raw_number,
	cleaned_number = excluded.cleaned_number,
	number = CASE
		WHEN ` + numberFrozenCondition + `
		THEN yamdc_job_tab.number
		ELSE excluded.number END,
	number_source = CASE
		WHEN ` + numberFrozenCondition + `
		THEN yamdc_job_tab.number_source
		ELSE excluded.number_source END,
	number_clean_status = CASE
		WHEN yamdc_job_tab.number_source = 'manual'
		THEN yamdc_job_tab.number_clean_status
		ELSE excluded.number_clean_status END,
	number_clean_confidence = CASE
		WHEN yamdc_job_tab.number_source = 'manual'
		THEN yamdc_job_tab.number_clean_confidence
		ELSE excluded.number_clean_confidence END,
	number_clean_warnings = CASE
		WHEN yamdc_job_tab.number_source = 'manual'
		THEN yamdc_job_tab.number_clean_warnings
		ELSE excluded.number_clean_warnings END,
	file_size = excluded.file_size,
	status = CASE
		WHEN yamdc_job_tab.status = 'done'
			OR yamdc_job_tab.deleted_at != 0
		THEN excluded.status
		ELSE yamdc_job_tab.status END,
	error_msg = CASE
		WHEN yamdc_job_tab.status = 'done'
			OR yamdc_job_tab.deleted_at != 0
		THEN '' ELSE yamdc_job_tab.error_msg END,
	retry_count = CASE
		WHEN yamdc_job_tab.status = 'done'
			OR yamdc_job_tab.deleted_at != 0
		THEN 0 ELSE yamdc_job_tab.retry_count END,
	scrape_started_at = CASE
		WHEN yamdc_job_tab.status = 'done'
			OR yamdc_job_tab.deleted_at != 0
		THEN 0
		ELSE yamdc_job_tab.scrape_started_at END,
	scrape_finished_at = CASE
		WHEN yamdc_job_tab.status = 'done'
			OR yamdc_job_tab.deleted_at != 0
		THEN 0
		ELSE yamdc_job_tab.scrape_finished_at END,
	reviewed_at = CASE
		WHEN yamdc_job_tab.status = 'done'
			OR yamdc_job_tab.deleted_at != 0
		THEN 0
		ELSE yamdc_job_tab.reviewed_at END,
	imported_at = CASE
		WHEN yamdc_job_tab.status = 'done'
			OR yamdc_job_tab.deleted_at != 0
		THEN 0
		ELSE yamdc_job_tab.imported_at END,
	deleted_at = 0,
	updated_at = CASE
		WHEN yamdc_job_tab.file_name != excluded.file_name
		OR yamdc_job_tab.file_ext != excluded.file_ext
		OR yamdc_job_tab.abs_path != excluded.abs_path
		OR yamdc_job_tab.raw_number != excluded.raw_number
		OR yamdc_job_tab.cleaned_number != excluded.cleaned_number
		OR (NOT ` + numberFrozenCondition + `
			AND yamdc_job_tab.conflict_key != excluded.conflict_key)
		OR (NOT ` + numberFrozenCondition + `
			AND yamdc_job_tab.number != excluded.number)
		OR (NOT ` + numberFrozenCondition + `
			AND yamdc_job_tab.number_source != excluded.number_source)
		OR (yamdc_job_tab.number_source != 'manual'
			AND yamdc_job_tab.number_clean_status
				!= excluded.number_clean_status)
		OR (yamdc_job_tab.number_source != 'manual'
			AND yamdc_job_tab.number_clean_confidence
				!= excluded.number_clean_confidence)
		OR (yamdc_job_tab.number_source != 'manual'
			AND yamdc_job_tab.number_clean_warnings
				!= excluded.number_clean_warnings)
		OR yamdc_job_tab.file_size != excluded.file_size
		OR yamdc_job_tab.status = 'done'
		OR yamdc_job_tab.deleted_at != 0
	THEN excluded.updated_at
	ELSE yamdc_job_tab.updated_at END
`

func (r *JobRepository) UpsertScannedJob(ctx context.Context, in UpsertJobInput) error {
	conflictNumber, err := r.resolveConflictNumber(ctx, in.RelPath, in.Number)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	conflictKey := jobdef.BuildConflictKey(conflictNumber, in.FileExt, in.FileName)
	_, err = r.db.ExecContext(ctx, upsertScannedJobSQL,
		uuid.NewString(), in.FileName, in.FileExt, conflictKey, in.RelPath, in.AbsPath,
		in.Number, in.RawNumber, in.CleanedNumber, in.NumberSource,
		in.NumberCleanStatus, in.NumberCleanConfidence, in.NumberCleanWarnings,
		in.FileSize, jobdef.StatusInit, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert scanned job failed: %w", err)
	}
	return nil
}

func buildListJobsFilter(status []jobdef.Status, keyword string) (string, []any) {
	where := ` WHERE deleted_at = 0`
	args := make([]any, 0, len(status)+4)
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
	return where, args
}

func scanJobRows(rows *sql.Rows) ([]jobdef.Job, error) {
	jobs := make([]jobdef.Job, 0, 16)
	for rows.Next() {
		var item jobdef.Job
		if err := rows.Scan(
			&item.ID, &item.JobUID, &item.FileName, &item.FileExt, &item.ConflictKey,
			&item.RelPath, &item.AbsPath, &item.Number, &item.RawNumber, &item.CleanedNumber,
			&item.NumberSource, &item.NumberCleanStatus, &item.NumberCleanConfidence,
			&item.NumberCleanWarnings, &item.FileSize, &item.Status, &item.ErrorMsg,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job failed: %w", err)
		}
		jobs = append(jobs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs failed: %w", err)
	}
	return jobs, nil
}

func (r *JobRepository) ListJobs(
	ctx context.Context, status []jobdef.Status, keyword string, page, pageSize int,
) (*ListJobsResult, error) {
	if page <= 0 {
		page = 1
	}
	all := pageSize <= 0
	if !all && pageSize > 200 {
		pageSize = 200
	}
	where, args := buildListJobsFilter(status, keyword)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM yamdc_job_tab`+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count jobs failed: %w", err)
	}
	//nolint:gosec // where clause is built from parameterized placeholders, not user input
	query := `SELECT id, job_uid, file_name, file_ext,
		conflict_key, rel_path, abs_path, number,
		raw_number, cleaned_number, number_source,
		number_clean_status, number_clean_confidence,
		number_clean_warnings, file_size, status,
		error_msg, created_at, updated_at
		FROM yamdc_job_tab` + where +
		` ORDER BY created_at DESC, id DESC`
	queryArgs := append([]any{}, args...)
	if !all {
		query += ` LIMIT ? OFFSET ?`
		queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	} else {
		pageSize = total
	}
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list jobs failed: %w", err)
	}
	defer func() { _ = rows.Close() }()
	jobs, err := scanJobRows(rows)
	if err != nil {
		return nil, err
	}
	return &ListJobsResult{Items: jobs, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *JobRepository) GetByID(ctx context.Context, id int64) (*jobdef.Job, error) {
	var item jobdef.Job
	err := r.db.QueryRowContext(ctx, `
		SELECT id, job_uid, file_name, file_ext, conflict_key, rel_path, abs_path, number, raw_number, cleaned_number,
		       number_source, number_clean_status, number_clean_confidence, number_clean_warnings,
		       file_size, status, error_msg, created_at, updated_at
		FROM yamdc_job_tab
		WHERE id = ? AND deleted_at = 0
	`, id).Scan(
		&item.ID,
		&item.JobUID,
		&item.FileName,
		&item.FileExt,
		&item.ConflictKey,
		&item.RelPath,
		&item.AbsPath,
		&item.Number,
		&item.RawNumber,
		&item.CleanedNumber,
		&item.NumberSource,
		&item.NumberCleanStatus,
		&item.NumberCleanConfidence,
		&item.NumberCleanWarnings,
		&item.FileSize,
		&item.Status,
		&item.ErrorMsg,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job failed: %w", err)
	}
	return &item, nil
}

func (r *JobRepository) UpdateNumber(
	ctx context.Context,
	id int64,
	number,
	source,
	cleanStatus,
	cleanConfidence,
	cleanWarnings string,
) error {
	var fileExt string
	var fileName string
	err := r.db.QueryRowContext(
		ctx,
		`SELECT file_ext,
		file_name FROM yamdc_job_tab WHERE id = ? AND deleted_at = 0`,
		id,
	).Scan(&fileExt, &fileName)
	if err != nil {
		return fmt.Errorf("load job before number update failed: %w", err)
	}
	conflictKey := jobdef.BuildConflictKey(number, fileExt, fileName)
	_, err = r.db.ExecContext(ctx, `
		UPDATE yamdc_job_tab
		SET number = ?, number_source = ?, number_clean_status = ?, number_clean_confidence = ?, number_clean_warnings = ?,
			conflict_key = ?, updated_at = ?
		WHERE id = ? AND deleted_at = 0
	`, number, source, cleanStatus, cleanConfidence, cleanWarnings, conflictKey, time.Now().UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("update job number failed: %w", err)
	}
	return nil
}

func (r *JobRepository) UpdateSourcePath(
	ctx context.Context,
	id int64,
	fileName,
	fileExt,
	relPath,
	absPath string,
) error {
	var numberText string
	err := r.db.QueryRowContext(
		ctx,
		`SELECT number FROM yamdc_job_tab WHERE id = ? AND deleted_at = 0`,
		id,
	).Scan(&numberText)
	if err != nil {
		return fmt.Errorf("load job before source path update failed: %w", err)
	}
	conflictKey := jobdef.BuildConflictKey(numberText, fileExt, fileName)
	_, err = r.db.ExecContext(ctx, `
		UPDATE yamdc_job_tab
		SET file_name = ?, file_ext = ?, conflict_key = ?, rel_path = ?, abs_path = ?, updated_at = ?
		WHERE id = ? AND deleted_at = 0
	`, fileName, fileExt, conflictKey, relPath, absPath, time.Now().UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("update job source path failed: %w", err)
	}
	return nil
}

func (r *JobRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	from []jobdef.Status,
	to jobdef.Status,
	errMsg string,
) (bool, error) {
	query := `UPDATE yamdc_job_tab SET status = ?, error_msg = ?, updated_at = ? WHERE id = ?`
	args := make([]any, 0, len(from)+4)
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

func (r *JobRepository) ListActiveJobsByConflictKeys(ctx context.Context, keys []string) ([]jobdef.Job, error) {
	filtered := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, key)
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	sort.Strings(filtered)
	placeholders := make([]string, 0, len(filtered))
	args := make([]any, 0, len(filtered))
	for _, key := range filtered {
		placeholders = append(placeholders, "?")
		args = append(args, key)
	}
	//nolint:gosec // placeholders are "?" literals, not user input
	query := `SELECT id, rel_path, conflict_key, status FROM yamdc_job_tab` +
		` WHERE deleted_at = 0 AND status != ? AND conflict_key IN (` +
		strings.Join(placeholders, ",") + `)`
	rows, err := r.db.QueryContext(ctx, query, append([]any{jobdef.StatusDone}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("list active jobs by conflict keys failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	items := make([]jobdef.Job, 0, len(filtered))
	for rows.Next() {
		var item jobdef.Job
		if err := rows.Scan(&item.ID, &item.RelPath, &item.ConflictKey, &item.Status); err != nil {
			return nil, fmt.Errorf("scan active job by conflict key failed: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active jobs by conflict key failed: %w", err)
	}
	return items, nil
}
