package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxxsen/yamdc/internal/repository"
)

var defaultMediaSuffix = []string{
	".mp4", ".wmv", ".flv", ".mpeg", ".m2ts", ".mts", ".mpe", ".mpg", ".m4v",
	".avi", ".mkv", ".rmvb", ".ts", ".mov", ".rm", ".strm",
}

type Service struct {
	scanDir string
	repo    *repository.JobRepository
	extMap  map[string]struct{}
}

func New(scanDir string, extraMediaExts []string, repo *repository.JobRepository) *Service {
	extMap := make(map[string]struct{}, len(defaultMediaSuffix)+len(extraMediaExts))
	for _, item := range defaultMediaSuffix {
		extMap[strings.ToLower(item)] = struct{}{}
	}
	for _, item := range extraMediaExts {
		extMap[strings.ToLower(item)] = struct{}{}
	}
	return &Service{
		scanDir: scanDir,
		repo:    repo,
		extMap:  extMap,
	}
}

func (s *Service) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		_ = s.Scan(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.Scan(ctx)
			}
		}
	}()
}

func (s *Service) Scan(ctx context.Context) error {
	entries := make([]repository.UpsertJobInput, 0, 32)
	err := filepath.Walk(s.scanDir, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !s.isMediaFile(path) {
			return nil
		}
		relPath, err := filepath.Rel(s.scanDir, path)
		if err != nil {
			return fmt.Errorf("resolve rel path failed: %w", err)
		}
		fileName := filepath.Base(path)
		ext := filepath.Ext(fileName)
		number := strings.TrimSuffix(fileName, ext)
		entries = append(entries, repository.UpsertJobInput{
			FileName: fileName,
			FileExt:  ext,
			RelPath:  filepath.ToSlash(relPath),
			AbsPath:  path,
			Number:   number,
			FileSize: info.Size(),
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk scan dir failed: %w", err)
	}
	for _, item := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := s.repo.UpsertScannedJob(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) isMediaFile(path string) bool {
	_, ok := s.extMap[strings.ToLower(filepath.Ext(path))]
	return ok
}
