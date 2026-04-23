package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/repository"
)

var (
	errScanAlreadyRunning  = errors.New("scan is already running")
	errMarkReviewingFailed = errors.New("mark reviewing job as failed failed")
)

type Service struct {
	scanDir string
	repo    *repository.JobRepository
	extMap  map[string]struct{}
	cleaner movieidcleaner.Cleaner

	mu      sync.Mutex
	scaning bool
}

func New(
	scanDir string,
	extraMediaExts []string,
	repo *repository.JobRepository,
	cleaner movieidcleaner.Cleaner,
) *Service {
	extMap := make(map[string]struct{}, len(enum.DefaultMediaSuffixes)+len(extraMediaExts))
	for _, item := range enum.DefaultMediaSuffixes {
		extMap[strings.ToLower(item)] = struct{}{}
	}
	for _, item := range extraMediaExts {
		extMap[strings.ToLower(item)] = struct{}{}
	}
	return &Service{
		scanDir: scanDir,
		repo:    repo,
		extMap:  extMap,
		cleaner: cleaner,
	}
}

func (s *Service) buildScanEntry(path string, info fs.FileInfo) (repository.UpsertJobInput, error) {
	relPath, err := filepath.Rel(s.scanDir, path)
	if err != nil {
		return repository.UpsertJobInput{}, fmt.Errorf("resolve rel path failed: %w", err)
	}
	fileName := filepath.Base(path)
	ext := filepath.Ext(fileName)
	rawNumber := strings.TrimSuffix(fileName, ext)
	number := rawNumber
	cleanedNumber := ""
	numberSource := "raw"
	numberCleanStatus := ""
	numberCleanConfidence := ""
	numberCleanWarnings := ""
	if s.cleaner != nil {
		res, err := s.cleaner.Clean(rawNumber)
		if err != nil {
			return repository.UpsertJobInput{}, fmt.Errorf("clean scan number failed: %w", err)
		}
		if res != nil {
			cleanedNumber = res.Normalized
			numberCleanStatus = string(res.Status)
			numberCleanConfidence = string(res.Confidence)
			numberCleanWarnings = strings.Join(res.Warnings, "; ")
		}
		if res != nil && res.Normalized != "" {
			number = res.Normalized
			numberSource = "cleaner"
		}
	}
	return repository.UpsertJobInput{
		FileName: fileName, FileExt: ext,
		RelPath: filepath.ToSlash(relPath), AbsPath: path,
		Number: number, RawNumber: rawNumber, CleanedNumber: cleanedNumber,
		NumberSource: numberSource, NumberCleanStatus: numberCleanStatus,
		NumberCleanConfidence: numberCleanConfidence, NumberCleanWarnings: numberCleanWarnings,
		FileSize: info.Size(),
	}, nil
}

func (s *Service) upsertEntries(ctx context.Context, entries []repository.UpsertJobInput) error {
	for _, item := range entries {
		select {
		case <-ctx.Done():
			return fmt.Errorf("scan context done: %w", ctx.Err())
		default:
		}
		if err := s.repo.UpsertScannedJob(ctx, item); err != nil {
			return fmt.Errorf("upsert scanned job: %w", err)
		}
	}
	return nil
}

// Scan 是对外的唯一入口: 手动触发由 web handler 调 (POST /api/jobs/scan),
// 将来需要做定时扫描, 在 bootstrap 里往 cronscheduler 注册一个调本方法
// 的 Job 即可, Service 本身不再自管 goroutine/ticker — 1.5 清理了 Start/Wait
// 这对历史接口后就是这个形状。
func (s *Service) Scan(ctx context.Context) error {
	if !s.claimScan() {
		return errScanAlreadyRunning
	}
	defer s.finishScan()
	entries := make([]repository.UpsertJobInput, 0, 32)
	err := filepath.Walk(s.scanDir, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !s.isMediaFile(path) {
			return nil
		}
		entry, err := s.buildScanEntry(path, info)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk scan dir failed: %w", err)
	}
	if err := s.upsertEntries(ctx, entries); err != nil {
		return err
	}
	return s.cleanupMissingJobs(ctx, entries)
}

func (s *Service) claimScan() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scaning {
		return false
	}
	s.scaning = true
	return true
}

func (s *Service) finishScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scaning = false
}

func (s *Service) isMediaFile(path string) bool {
	_, ok := s.extMap[strings.ToLower(filepath.Ext(path))]
	return ok
}

func (s *Service) cleanupMissingJobs(ctx context.Context, entries []repository.UpsertJobInput) error {
	activePaths := make(map[string]struct{}, len(entries))
	for _, item := range entries {
		activePaths[item.RelPath] = struct{}{}
	}
	summaries, err := s.repo.ListActiveJobSummaries(
		ctx,
		[]jobdef.Status{jobdef.StatusInit, jobdef.StatusFailed, jobdef.StatusReviewing},
	)
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}
	for _, job := range summaries {
		if _, ok := activePaths[job.RelPath]; ok {
			continue
		}
		switch job.Status { //nolint:exhaustive // only reviewing needs special handling, others soft-delete
		case jobdef.StatusReviewing:
			ok, err := s.repo.UpdateStatus(
				ctx,
				job.ID,
				[]jobdef.Status{jobdef.StatusReviewing},
				jobdef.StatusFailed,
				"source file missing, unable to import",
			)
			if err != nil {
				return fmt.Errorf("update status for job %d: %w", job.ID, err)
			}
			if !ok {
				return fmt.Errorf("id %d: %w", job.ID, errMarkReviewingFailed)
			}
		default:
			if err := s.repo.SoftDelete(ctx, job.ID); err != nil {
				return fmt.Errorf("soft delete job %d: %w", job.ID, err)
			}
		}
	}
	return nil
}
