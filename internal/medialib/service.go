package medialib

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

const (
	TaskSync = "media_library_sync"
	TaskMove = "media_library_move"
)

type Service struct {
	db         *sql.DB
	libraryDir string
	saveDir    string

	mu          sync.Mutex
	syncRunning bool
	moveRunning bool
}

type ListItemsOptions struct {
	Keyword    string
	Year       string
	SizeFilter string
	Sort       string
	Order      string
}

func NewService(db *sql.DB, libraryDir string, saveDir string) *Service {
	return &Service{
		db:         db,
		libraryDir: libraryDir,
		saveDir:    saveDir,
	}
}

func (s *Service) IsConfigured() bool {
	return s.libraryDir != ""
}

func (s *Service) Start(ctx context.Context) {
	if s.db == nil {
		return
	}
	if err := s.recoverTaskStates(ctx); err != nil {
		logutil.GetLogger(ctx).Error("recover media library task states failed", zap.Error(err))
	}
}

func (s *Service) ListItems(ctx context.Context, options ListItemsOptions) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, item_json, created_at
		FROM yamdc_media_library_tab
	`)
	if err != nil {
		return nil, fmt.Errorf("list media library items failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	items := make([]Item, 0, 32)
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	year := strings.TrimSpace(options.Year)
	for rows.Next() {
		var id int64
		var raw string
		var createdAt int64
		if err := rows.Scan(&id, &raw, &createdAt); err != nil {
			return nil, fmt.Errorf("scan media library item failed: %w", err)
		}
		var item Item
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode media library item failed: %w", err)
		}
		item.ID = id
		item.CreatedAt = createdAt
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				item.Title,
				item.Number,
				item.Name,
			}, " "))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		if year != "" && year != "all" && releaseYear(item.ReleaseDate) != year {
			continue
		}
		if !matchSizeFilter(item.TotalSize, options.SizeFilter) {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media library items failed: %w", err)
	}
	sortItems(items, options.Sort, options.Order)
	return items, nil
}

func (s *Service) GetDetail(ctx context.Context, id int64) (*Detail, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT detail_json
		FROM yamdc_media_library_tab
		WHERE id = ?
	`, id).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("get media library detail failed: %w", err)
	}
	var detail Detail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		return nil, fmt.Errorf("decode media library detail failed: %w", err)
	}
	detail.Item.ID = id
	return &detail, nil
}

func (s *Service) GetDetailByRelPath(ctx context.Context, relPath string) (*Detail, error) {
	var id int64
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, detail_json
		FROM yamdc_media_library_tab
		WHERE rel_path = ?
	`, relPath).Scan(&id, &raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("get media library detail by rel path failed: %w", err)
	}
	var detail Detail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		return nil, fmt.Errorf("decode media library detail failed: %w", err)
	}
	detail.Item.ID = id
	return &detail, nil
}

func (s *Service) UpdateItem(ctx context.Context, id int64, meta Meta) (*Detail, error) {
	detail, err := s.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	absPath := filepath.Join(s.libraryDir, filepath.FromSlash(detail.Item.RelPath))
	next, err := s.updateRootItem(s.libraryDir, detail, absPath, meta)
	if err != nil {
		return nil, err
	}
	if err := s.upsertDetail(ctx, next); err != nil {
		return nil, err
	}
	next.Item.ID = id
	next.Item.CreatedAt = detail.Item.CreatedAt
	return next, nil
}

func (s *Service) ReplaceAsset(ctx context.Context, id int64, variantKey string, kind string, originalName string, data []byte) (*Detail, error) {
	detail, err := s.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	absPath := filepath.Join(s.libraryDir, filepath.FromSlash(detail.Item.RelPath))
	next, err := s.replaceRootArtwork(s.libraryDir, detail, absPath, variantKey, kind, originalName, data)
	if err != nil {
		return nil, err
	}
	if err := s.upsertDetail(ctx, next); err != nil {
		return nil, err
	}
	next.Item.ID = id
	next.Item.CreatedAt = detail.Item.CreatedAt
	return next, nil
}

func (s *Service) DeleteFile(ctx context.Context, id int64, fileRelPath string) (*Detail, error) {
	detail, err := s.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	next, err := s.deleteRootFile(s.libraryDir, detail.Item.RelPath, fileRelPath)
	if err != nil {
		return nil, err
	}
	if err := s.upsertDetail(ctx, next); err != nil {
		return nil, err
	}
	next.Item.ID = id
	next.Item.CreatedAt = detail.Item.CreatedAt
	return next, nil
}

func (s *Service) ResolveLibraryPath(raw string) (string, string, error) {
	return s.resolveRootPath(s.libraryDir, raw)
}

func (s *Service) TriggerFullSync(ctx context.Context) error {
	logger := logutil.GetLogger(ctx).With(zap.String("task", TaskSync), zap.String("reason", "manual"))
	if !s.IsConfigured() {
		logger.Warn("media library sync skipped because library dir is not configured")
		return fmt.Errorf("library dir is not configured")
	}
	if s.isMoveRunning() {
		logger.Warn("media library sync skipped because move task is running")
		return fmt.Errorf("move to media library is running")
	}
	if syncState, err := s.getTaskState(ctx, TaskSync); err == nil && syncState.Status == "running" {
		logger.Warn("media library sync skipped because sync task is already running")
		return fmt.Errorf("media library sync is already running")
	}
	logger.Info("media library sync triggered")
	go func() {
		_ = s.runFullSync(context.WithoutCancel(ctx), "manual")
	}()
	return nil
}

func (s *Service) recoverTaskStates(ctx context.Context) error {
	for _, taskKey := range []string{TaskSync, TaskMove} {
		if err := s.recoverTaskState(ctx, taskKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recoverTaskState(ctx context.Context, taskKey string) error {
	state, err := s.getTaskState(ctx, taskKey)
	if err != nil {
		return err
	}
	if state.Status != "running" {
		return nil
	}
	now := time.Now().UnixMilli()
	state.Status = "failed"
	state.Message = "server restarted while task running"
	state.FinishedAt = now
	state.UpdatedAt = now
	if err := s.saveTaskState(ctx, state); err != nil {
		return err
	}
	logutil.GetLogger(ctx).Warn("recover media library task state from running to failed", zap.String("task", taskKey))
	return nil
}

func (s *Service) TriggerMove(ctx context.Context) error {
	logger := logutil.GetLogger(ctx).With(zap.String("task", TaskMove))
	if !s.IsConfigured() {
		logger.Warn("move to media library skipped because library dir is not configured")
		return fmt.Errorf("library dir is not configured")
	}
	if s.saveDir == "" {
		logger.Warn("move to media library skipped because save dir is not configured")
		return fmt.Errorf("save dir is not configured")
	}
	if s.isSyncRunning() {
		logger.Warn("move to media library skipped because sync task is running")
		return fmt.Errorf("media library sync is running")
	}
	if !s.claimMove() {
		logger.Warn("move to media library skipped because move task is already running")
		return fmt.Errorf("move to media library is already running")
	}
	logger.Info("move to media library triggered")
	go func() {
		_ = s.runMove(context.WithoutCancel(ctx))
	}()
	return nil
}

func (s *Service) IsMoveRunning() bool {
	return s.isMoveRunning()
}

func (s *Service) GetStatusSnapshot(ctx context.Context) (*StatusSnapshot, error) {
	syncState, err := s.getTaskState(ctx, TaskSync)
	if err != nil {
		return nil, err
	}
	moveState, err := s.getTaskState(ctx, TaskMove)
	if err != nil {
		return nil, err
	}
	return &StatusSnapshot{
		Configured: s.IsConfigured(),
		Sync:       syncState,
		Move:       moveState,
	}, nil
}

func (s *Service) runFullSync(ctx context.Context, reason string) error {
	logger := logutil.GetLogger(ctx).With(zap.String("task", TaskSync), zap.String("reason", reason))
	if !s.IsConfigured() {
		return nil
	}
	if reason != "move" && s.isMoveRunning() {
		return nil
	}
	if !s.claimSync() {
		return nil
	}
	defer s.finishSync()

	itemDirs, err := s.listRootItemDirs(s.libraryDir)
	if err != nil {
		logger.Error("list media library directories failed", zap.Error(err))
		_ = s.saveTaskState(ctx, TaskState{
			TaskKey:    TaskSync,
			Status:     "failed",
			Message:    err.Error(),
			UpdatedAt:  time.Now().UnixMilli(),
			FinishedAt: time.Now().UnixMilli(),
		})
		return err
	}
	logger.Info("media library sync started", zap.Int("total", len(itemDirs)))
	startedAt := time.Now().UnixMilli()
	state := TaskState{
		TaskKey:    TaskSync,
		Status:     "running",
		Total:      len(itemDirs),
		Processed:  0,
		Current:    "",
		Message:    "同步媒体库中",
		StartedAt:  startedAt,
		FinishedAt: 0,
		UpdatedAt:  startedAt,
	}
	_ = s.saveTaskState(ctx, state)
	keep := make(map[string]struct{}, len(itemDirs))
	successCount := 0
	errorCount := 0
	for index, absPath := range itemDirs {
		relPath, err := filepath.Rel(s.libraryDir, absPath)
		if err != nil {
			logger.Warn("resolve media library relative path failed", zap.String("abs_path", absPath), zap.Error(err))
			errorCount++
			continue
		}
		relPath = filepath.ToSlash(relPath)
		detail, err := s.readRootDetail(s.libraryDir, relPath, absPath)
		if err != nil {
			logger.Warn("read media library detail failed", zap.String("rel_path", relPath), zap.Error(err))
			errorCount++
		} else if err := s.upsertDetail(ctx, detail); err != nil {
			logger.Warn("upsert media library detail failed", zap.String("rel_path", relPath), zap.Error(err))
			errorCount++
		} else {
			keep[relPath] = struct{}{}
			successCount++
		}
		state.Processed = index + 1
		state.SuccessCount = successCount
		state.ErrorCount = errorCount
		state.Current = relPath
		state.UpdatedAt = time.Now().UnixMilli()
		_ = s.saveTaskState(ctx, state)
	}
	if err := s.deleteMissing(ctx, keep); err != nil {
		logger.Warn("delete stale media library items failed", zap.Error(err))
		errorCount++
	}
	state.Status = "completed"
	state.Message = fmt.Sprintf("媒体库同步完成 (%s)", reason)
	state.ErrorCount = errorCount
	state.SuccessCount = successCount
	state.Current = ""
	state.UpdatedAt = time.Now().UnixMilli()
	state.FinishedAt = state.UpdatedAt
	_ = s.saveTaskState(ctx, state)
	logger.Info("media library sync completed",
		zap.Int("total", state.Total),
		zap.Int("success_count", state.SuccessCount),
		zap.Int("error_count", state.ErrorCount),
	)
	return nil
}

func (s *Service) runMove(ctx context.Context) error {
	logger := logutil.GetLogger(ctx).With(zap.String("task", TaskMove))
	defer s.finishMove()

	itemDirs, err := s.listRootItemDirs(s.saveDir)
	if err != nil {
		logger.Error("list save directories before move failed", zap.Error(err))
		_ = s.saveTaskState(ctx, TaskState{
			TaskKey:    TaskMove,
			Status:     "failed",
			Message:    err.Error(),
			UpdatedAt:  time.Now().UnixMilli(),
			FinishedAt: time.Now().UnixMilli(),
		})
		return err
	}
	logger.Info("move to media library started", zap.Int("total", len(itemDirs)))
	startedAt := time.Now().UnixMilli()
	state := TaskState{
		TaskKey:   TaskMove,
		Status:    "running",
		Total:     len(itemDirs),
		Message:   "移动到媒体库中",
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	_ = s.saveTaskState(ctx, state)
	successCount := 0
	conflictCount := 0
	errorCount := 0
	for index, absPath := range itemDirs {
		relPath, err := filepath.Rel(s.saveDir, absPath)
		if err != nil {
			logger.Warn("resolve save relative path failed", zap.String("abs_path", absPath), zap.Error(err))
			errorCount++
			continue
		}
		relPath = filepath.ToSlash(relPath)
		targetAbs := filepath.Join(s.libraryDir, filepath.FromSlash(relPath))
		if _, err := os.Stat(targetAbs); err == nil {
			conflictCount++
		} else if !os.IsNotExist(err) {
			logger.Warn("check target media library path failed", zap.String("rel_path", relPath), zap.Error(err))
			errorCount++
		} else {
			if err := os.MkdirAll(filepath.Dir(targetAbs), 0755); err != nil {
				logger.Warn("create media library parent directory failed", zap.String("rel_path", relPath), zap.Error(err))
				errorCount++
			} else if err := moveDirectory(absPath, targetAbs); err != nil {
				logger.Warn("move directory to media library failed", zap.String("rel_path", relPath), zap.Error(err))
				errorCount++
			} else {
				successCount++
			}
		}
		state.Processed = index + 1
		state.SuccessCount = successCount
		state.ConflictCount = conflictCount
		state.ErrorCount = errorCount
		state.Current = relPath
		state.UpdatedAt = time.Now().UnixMilli()
		_ = s.saveTaskState(ctx, state)
	}
	_ = s.runFullSync(ctx, "move")
	state.Status = "completed"
	state.Message = "移动到媒体库完成"
	state.Current = ""
	state.UpdatedAt = time.Now().UnixMilli()
	state.FinishedAt = state.UpdatedAt
	state.SuccessCount = successCount
	state.ConflictCount = conflictCount
	state.ErrorCount = errorCount
	_ = s.saveTaskState(ctx, state)
	logger.Info("move to media library completed",
		zap.Int("total", state.Total),
		zap.Int("success_count", state.SuccessCount),
		zap.Int("conflict_count", state.ConflictCount),
		zap.Int("error_count", state.ErrorCount),
	)
	return nil
}

func moveDirectory(src string, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if linkErr, ok := err.(*os.LinkError); !ok || linkErr.Err != syscall.EXDEV {
		return err
	}
	if err := copyDirectory(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyDirectory(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = in.Close()
		}()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		if _, err := out.ReadFrom(in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}

func (s *Service) upsertDetail(ctx context.Context, detail *Detail) error {
	itemRaw, err := json.Marshal(detail.Item)
	if err != nil {
		return fmt.Errorf("marshal media library item failed: %w", err)
	}
	detailRaw, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal media library detail failed: %w", err)
	}
	now := time.Now().UnixMilli()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO yamdc_media_library_tab (
			rel_path, title, release_date, updated_at, poster_path, cover_path, item_json, detail_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rel_path) DO UPDATE SET
			title = excluded.title,
			release_date = excluded.release_date,
			updated_at = excluded.updated_at,
			poster_path = excluded.poster_path,
			cover_path = excluded.cover_path,
			item_json = excluded.item_json,
			detail_json = excluded.detail_json
	`, detail.Item.RelPath, detail.Item.Title, detail.Item.ReleaseDate, detail.Item.UpdatedAt, detail.Item.PosterPath, detail.Item.CoverPath, string(itemRaw), string(detailRaw), now)
	if err != nil {
		return fmt.Errorf("upsert media library detail failed: %w", err)
	}
	return nil
}

func (s *Service) deleteMissing(ctx context.Context, keep map[string]struct{}) error {
	rows, err := s.db.QueryContext(ctx, `SELECT rel_path FROM yamdc_media_library_tab`)
	if err != nil {
		return fmt.Errorf("list existing media library rel paths failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	paths := make([]string, 0, 32)
	for rows.Next() {
		var relPath string
		if err := rows.Scan(&relPath); err != nil {
			return fmt.Errorf("scan media library rel path failed: %w", err)
		}
		if _, ok := keep[relPath]; !ok {
			paths = append(paths, relPath)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate media library rel paths failed: %w", err)
	}
	for _, relPath := range paths {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM yamdc_media_library_tab WHERE rel_path = ?`, relPath); err != nil {
			return fmt.Errorf("delete stale media library row failed: %w", err)
		}
	}
	return nil
}

func (s *Service) getTaskState(ctx context.Context, taskKey string) (TaskState, error) {
	state := TaskState{TaskKey: taskKey, Status: "idle"}
	err := s.db.QueryRowContext(ctx, `
		SELECT status, total, processed, success_count, conflict_count, error_count, current, message, started_at, finished_at, updated_at
		FROM yamdc_task_state_tab
		WHERE task_key = ?
	`, taskKey).Scan(
		&state.Status,
		&state.Total,
		&state.Processed,
		&state.SuccessCount,
		&state.ConflictCount,
		&state.ErrorCount,
		&state.Current,
		&state.Message,
		&state.StartedAt,
		&state.FinishedAt,
		&state.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return state, nil
		}
		return TaskState{}, fmt.Errorf("get task state failed: %w", err)
	}
	return state, nil
}

func (s *Service) saveTaskState(ctx context.Context, state TaskState) error {
	if state.TaskKey == "" {
		return nil
	}
	if state.UpdatedAt == 0 {
		state.UpdatedAt = time.Now().UnixMilli()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO yamdc_task_state_tab (
			task_key, status, total, processed, success_count, conflict_count, error_count, current, message, started_at, finished_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_key) DO UPDATE SET
			status = excluded.status,
			total = excluded.total,
			processed = excluded.processed,
			success_count = excluded.success_count,
			conflict_count = excluded.conflict_count,
			error_count = excluded.error_count,
			current = excluded.current,
			message = excluded.message,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			updated_at = excluded.updated_at
	`, state.TaskKey, state.Status, state.Total, state.Processed, state.SuccessCount, state.ConflictCount, state.ErrorCount, state.Current, state.Message, state.StartedAt, state.FinishedAt, state.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save task state failed: %w", err)
	}
	return nil
}

func (s *Service) claimSync() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syncRunning {
		return false
	}
	s.syncRunning = true
	return true
}

func (s *Service) finishSync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncRunning = false
}

func (s *Service) claimMove() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.moveRunning {
		return false
	}
	s.moveRunning = true
	return true
}

func (s *Service) finishMove() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moveRunning = false
}

func (s *Service) isMoveRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moveRunning
}

func (s *Service) isSyncRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncRunning
}

func releaseYear(value string) string {
	for start := 0; start+4 <= len(value); start++ {
		chunk := value[start : start+4]
		valid := true
		for _, char := range chunk {
			if char < '0' || char > '9' {
				valid = false
				break
			}
		}
		if valid {
			return chunk
		}
	}
	return ""
}

func matchSizeFilter(totalSize int64, sizeFilter string) bool {
	gb := float64(totalSize) / float64(1024*1024*1024)
	switch sizeFilter {
	case "", "all":
		return true
	case "lt-1":
		return gb < 1
	case "1-2":
		return gb >= 1 && gb < 2
	case "2-5":
		return gb >= 2 && gb < 5
	case "lt-5":
		return gb < 5
	case "5-10":
		return gb >= 5 && gb < 10
	case "10-20":
		return gb >= 10 && gb < 20
	case "5-20":
		return gb >= 5 && gb < 20
	case "20-50":
		return gb >= 20 && gb < 50
	case "50-plus":
		return gb >= 50
	default:
		return true
	}
}

func sortItems(items []Item, sortMode string, order string) {
	if sortMode == "" {
		sortMode = "ingested"
	}
	desc := order != "asc"
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		compare := 0
		switch sortMode {
		case "title":
			leftTitle := firstNonEmpty(left.Title, left.Name)
			rightTitle := firstNonEmpty(right.Title, right.Name)
			if leftTitle != rightTitle {
				if leftTitle < rightTitle {
					compare = -1
				} else {
					compare = 1
				}
			}
		case "size":
			if left.TotalSize != right.TotalSize {
				if left.TotalSize < right.TotalSize {
					compare = -1
				} else {
					compare = 1
				}
			}
		case "year":
			leftYear := releaseYear(left.ReleaseDate)
			rightYear := releaseYear(right.ReleaseDate)
			if leftYear != rightYear {
				if leftYear < rightYear {
					compare = -1
				} else {
					compare = 1
				}
			}
		case "ingested":
			if left.CreatedAt != right.CreatedAt {
				if left.CreatedAt < right.CreatedAt {
					compare = -1
				} else {
					compare = 1
				}
			}
		default:
			if left.UpdatedAt != right.UpdatedAt {
				if left.UpdatedAt < right.UpdatedAt {
					compare = -1
				} else {
					compare = 1
				}
			}
		}
		if compare == 0 && left.UpdatedAt != right.UpdatedAt {
			if left.UpdatedAt < right.UpdatedAt {
				compare = -1
			} else {
				compare = 1
			}
		}
		if compare == 0 && left.ID != right.ID {
			if left.ID < right.ID {
				compare = -1
			} else {
				compare = 1
			}
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}
