package job

import (
	"context"
	"encoding/json"
	"fmt"
	stdimage "image"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/capture"
	imgutil "github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
	"go.uber.org/zap"
)

type Service struct {
	jobRepo     *repository.JobRepository
	logRepo     *repository.LogRepository
	scrapeRepo  *repository.ScrapeDataRepository
	capture     *capture.Capture
	storage     store.IStorage
	importGuard func(context.Context) error

	mu      sync.Mutex
	running map[int64]struct{}
}

type JobConflict struct {
	Reason string
	Target string
}

func NewService(
	jobRepo *repository.JobRepository,
	logRepo *repository.LogRepository,
	scrapeRepo *repository.ScrapeDataRepository,
	cap *capture.Capture,
	storage store.IStorage,
) *Service {
	return &Service{
		jobRepo:    jobRepo,
		logRepo:    logRepo,
		scrapeRepo: scrapeRepo,
		capture:    cap,
		storage:    storage,
		running:    make(map[int64]struct{}),
	}
}

func requiresManualNumberReview(j *jobdef.Job) bool {
	if j == nil {
		return false
	}
	if j.NumberSource == "manual" {
		return false
	}
	if j.NumberCleanStatus == "no_match" || j.NumberCleanStatus == "low_quality" {
		return true
	}
	return j.NumberCleanConfidence == "low"
}

func (s *Service) Run(ctx context.Context, jobID int64) error {
	return s.start(ctx, jobID, []jobdef.Status{jobdef.StatusInit})
}

func (s *Service) Rerun(ctx context.Context, jobID int64) error {
	return s.start(ctx, jobID, []jobdef.Status{jobdef.StatusFailed})
}

func (s *Service) ListLogs(ctx context.Context, jobID int64) ([]repository.LogItem, error) {
	return s.logRepo.ListByJobID(ctx, jobID, 500)
}

func (s *Service) GetScrapeData(ctx context.Context, jobID int64) (*repository.ScrapeData, error) {
	return s.scrapeRepo.GetByJobID(ctx, jobID)
}

func (s *Service) SetImportGuard(fn func(context.Context) error) {
	s.importGuard = fn
}

func (s *Service) UpdateNumber(ctx context.Context, jobID int64, input string) (*jobdef.Job, error) {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID), zap.String("number", strings.TrimSpace(input)))
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before number update failed", zap.Error(err))
		return nil, err
	}
	if j == nil {
		return nil, fmt.Errorf("job not found")
	}
	if j.Status != jobdef.StatusInit && j.Status != jobdef.StatusFailed {
		return nil, fmt.Errorf("job number can only be edited in init or failed status")
	}
	sourcePath, err := s.resolveJobSourcePath(ctx, j)
	if err != nil {
		return nil, err
	}
	fc, err := s.capture.ResolveFileContext(sourcePath, input)
	if err != nil {
		return nil, fmt.Errorf("validate number failed: %w", err)
	}
	numberText := fc.Number.GenerateFileName()
	if err := s.jobRepo.UpdateNumber(ctx, jobID, numberText, "manual", "success", "high", ""); err != nil {
		logger.Error("persist updated job number failed", zap.Error(err))
		return nil, err
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "number", "job number updated", numberText)
	logger.Info("job number updated", zap.String("normalized_number", numberText))
	updated, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if updated != nil {
		if conflict, err := s.GetJobConflict(ctx, updated); err == nil && conflict != nil {
			updated.ConflictReason = conflict.Reason
			updated.ConflictTarget = conflict.Target
		}
	}
	return updated, nil
}

func (s *Service) SaveReviewData(ctx context.Context, jobID int64, reviewData string) error {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID))
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before saving review data failed", zap.Error(err))
		return err
	}
	if j == nil {
		return fmt.Errorf("job not found")
	}
	if j.Status != jobdef.StatusReviewing {
		return fmt.Errorf("job is not in reviewing status")
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(reviewData), &meta); err != nil {
		return fmt.Errorf("invalid review json: %w", err)
	}
	if err := s.scrapeRepo.SaveReviewData(ctx, jobID, reviewData); err != nil {
		logger.Error("save review data failed", zap.Error(err))
		return err
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "review", "review data saved", "")
	logger.Info("review data saved", zap.String("number", meta.Number), zap.String("title", meta.Title))
	return nil
}

func (s *Service) CropPosterFromCover(ctx context.Context, jobID int64, x, y, width, height int) (*model.File, error) {
	logger := logutil.GetLogger(ctx).With(
		zap.Int64("job_id", jobID),
		zap.Int("x", x),
		zap.Int("y", y),
		zap.Int("width", width),
		zap.Int("height", height),
	)
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before poster crop failed", zap.Error(err))
		return nil, err
	}
	if j == nil {
		return nil, fmt.Errorf("job not found")
	}
	if j.Status != jobdef.StatusReviewing {
		return nil, fmt.Errorf("job is not in reviewing status")
	}
	data, err := s.scrapeRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("scrape data not found")
	}
	payload := data.RawData
	if data.ReviewData != "" {
		payload = data.ReviewData
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(payload), &meta); err != nil {
		return nil, fmt.Errorf("parse review meta failed: %w", err)
	}
	if meta.Cover == nil || meta.Cover.Key == "" {
		return nil, fmt.Errorf("cover not found")
	}
	raw, err := store.GetDataFrom(ctx, s.storage, meta.Cover.Key)
	if err != nil {
		return nil, fmt.Errorf("load cover failed: %w", err)
	}
	img, err := imgutil.LoadImage(raw)
	if err != nil {
		return nil, fmt.Errorf("decode cover failed: %w", err)
	}
	bounds := img.Bounds()
	rect := stdimage.Rect(x, y, x+width, y+height)
	if rect.Min.X < bounds.Min.X || rect.Min.Y < bounds.Min.Y || rect.Max.X > bounds.Max.X || rect.Max.Y > bounds.Max.Y {
		return nil, fmt.Errorf("crop rectangle out of bounds")
	}
	cropped, err := imgutil.CutImageViaRectangle(img, rect)
	if err != nil {
		return nil, fmt.Errorf("crop poster failed: %w", err)
	}
	croppedRaw, err := imgutil.WriteImageToBytes(cropped)
	if err != nil {
		return nil, fmt.Errorf("encode poster failed: %w", err)
	}
	key, err := store.AnonymousPutDataTo(ctx, s.storage, croppedRaw)
	if err != nil {
		return nil, fmt.Errorf("store poster failed: %w", err)
	}
	meta.Poster = &model.File{
		Name: "./poster.jpg",
		Key:  key,
	}
	reviewData, err := json.Marshal(&meta)
	if err != nil {
		return nil, fmt.Errorf("marshal review meta failed: %w", err)
	}
	if err := s.scrapeRepo.SaveReviewData(ctx, jobID, string(reviewData)); err != nil {
		logger.Error("save cropped poster review data failed", zap.Error(err))
		return nil, err
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "review", "poster cropped from cover", fmt.Sprintf("%d,%d,%d,%d", x, y, width, height))
	logger.Info("poster cropped from cover", zap.String("poster_key", meta.Poster.Key))
	return meta.Poster, nil
}

func (s *Service) Import(ctx context.Context, jobID int64) error {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID))
	if !s.claim(jobID) {
		logger.Warn("import skipped because job is already running")
		return fmt.Errorf("job is already running")
	}
	defer s.finish(jobID)

	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before import failed", zap.Error(err))
		return err
	}
	if j == nil {
		return fmt.Errorf("job not found")
	}
	if j.Status != jobdef.StatusReviewing {
		return fmt.Errorf("job is not in reviewing status")
	}
	if s.importGuard != nil {
		if err := s.importGuard(ctx); err != nil {
			logger.Warn("import blocked by guard", zap.Error(err))
			return err
		}
	}
	if conflict, err := s.GetJobConflict(ctx, j); err != nil {
		logger.Error("check job conflict before import failed", zap.Error(err))
		return err
	} else if conflict != nil {
		logger.Warn("import blocked by conflict", zap.String("reason", conflict.Reason), zap.String("target", conflict.Target))
		return fmt.Errorf("%s: %s", conflict.Reason, conflict.Target)
	}
	data, err := s.scrapeRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("scrape data not found")
	}
	payload := data.RawData
	if data.ReviewData != "" {
		payload = data.ReviewData
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(payload), &meta); err != nil {
		return fmt.Errorf("parse final meta failed: %w", err)
	}
	sourcePath, err := s.resolveJobSourcePath(ctx, j)
	if err != nil {
		return err
	}
	fc, err := s.capture.ResolveFileContext(sourcePath, j.Number)
	if err != nil {
		return fmt.Errorf("resolve file context failed: %w", err)
	}
	fc.Meta = &meta
	_ = s.logRepo.Add(ctx, jobID, "info", "import", "import started", "")
	logger.Info("import started", zap.String("number", j.Number))
	if err := s.capture.ImportMeta(ctx, fc); err != nil {
		_ = s.logRepo.Add(ctx, jobID, "error", "import", "import failed", err.Error())
		logger.Error("import failed", zap.Error(err))
		return err
	}
	if err := s.scrapeRepo.SaveFinalData(ctx, jobID, payload); err != nil {
		return err
	}
	if err := s.jobRepo.MarkDone(ctx, jobID); err != nil {
		return err
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "import", "import completed", fc.SaveDir)
	logger.Info("import completed", zap.String("save_dir", fc.SaveDir))
	return nil
}

func (s *Service) Delete(ctx context.Context, jobID int64) error {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID))
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before delete failed", zap.Error(err))
		return err
	}
	if j == nil {
		return fmt.Errorf("job not found")
	}
	switch j.Status {
	case jobdef.StatusInit, jobdef.StatusFailed, jobdef.StatusReviewing:
	default:
		return fmt.Errorf("job status does not allow delete")
	}

	if !s.claim(jobID) {
		logger.Warn("delete skipped because job is currently running")
		return fmt.Errorf("job is currently running")
	}
	defer s.finish(jobID)

	if err := os.Remove(j.AbsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete source file failed: %w", err)
	}
	if err := s.scrapeRepo.DeleteByJobID(ctx, jobID); err != nil {
		return err
	}
	if err := s.logRepo.DeleteByJobID(ctx, jobID); err != nil {
		return err
	}
	if err := s.jobRepo.SoftDelete(ctx, jobID); err != nil {
		logger.Error("soft delete job failed", zap.Error(err))
		return err
	}
	logger.Info("job deleted", zap.String("path", j.AbsPath))
	return nil
}

func (s *Service) Recover(ctx context.Context) error {
	return s.jobRepo.RecoverProcessingJobs(ctx)
}

func (s *Service) start(ctx context.Context, jobID int64, allowed []jobdef.Status) error {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	if j == nil {
		return fmt.Errorf("job not found")
	}
	if requiresManualNumberReview(j) {
		return fmt.Errorf("job number requires manual edit before scraping")
	}
	if conflict, err := s.GetJobConflict(ctx, j); err != nil {
		return err
	} else if conflict != nil {
		return fmt.Errorf("%s: %s", conflict.Reason, conflict.Target)
	}

	if !s.claim(jobID) {
		return fmt.Errorf("job is already running")
	}

	ok, err := s.jobRepo.UpdateStatus(ctx, jobID, allowed, jobdef.StatusProcessing, "")
	if err != nil {
		s.finish(jobID)
		return err
	}
	if !ok {
		s.finish(jobID)
		return fmt.Errorf("job status is not runnable")
	}
	go s.runOne(jobID)
	return nil
}

func (s *Service) runOne(jobID int64) {
	defer s.finish(jobID)
	ctx := context.Background()
	_ = s.logRepo.Add(ctx, jobID, "info", "job", "job started", "")

	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("load job failed: %v", err))
		return
	}
	if j == nil {
		return
	}
	sourcePath, err := s.resolveJobSourcePath(ctx, j)
	if err != nil {
		s.failJob(ctx, jobID, err.Error())
		return
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "prepare", "resolve file context", sourcePath)
	fc, err := s.capture.ResolveFileContext(sourcePath, j.Number)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("resolve file failed: %v", err))
		return
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "scrape", "scrape meta start", fc.Number.GetNumberID())
	if err := s.capture.ScrapeMeta(ctx, fc); err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("scrape meta failed: %v", err))
		return
	}
	raw, err := json.Marshal(fc.Meta)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("marshal meta failed: %v", err))
		return
	}
	source := ""
	if fc.Meta != nil {
		source = fc.Meta.ExtInfo.ScrapeInfo.Source
	}
	if err := s.scrapeRepo.UpsertRawData(ctx, jobID, source, string(raw)); err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("save scrape data failed: %v", err))
		return
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "scrape", "scrape meta completed", source)
	ok, err := s.jobRepo.UpdateStatus(ctx, jobID, []jobdef.Status{jobdef.StatusProcessing}, jobdef.StatusReviewing, "")
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("update reviewing failed: %v", err))
		return
	}
	if !ok {
		s.failJob(ctx, jobID, "job status changed unexpectedly")
		return
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "job", "job moved to reviewing", "")
}

func (s *Service) resolveJobSourcePath(ctx context.Context, j *jobdef.Job) (string, error) {
	if j == nil {
		return "", fmt.Errorf("job not found")
	}
	if j.AbsPath == "" {
		return "", fmt.Errorf("job source path is empty")
	}
	if info, err := os.Stat(j.AbsPath); err == nil && !info.IsDir() {
		return j.AbsPath, nil
	}
	dirCandidates := make([]string, 0, 3)
	appendDir := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		for _, item := range dirCandidates {
			if item == candidate {
				return
			}
		}
		dirCandidates = append(dirCandidates, candidate)
	}
	appendDir(filepath.Dir(j.AbsPath))
	if s.capture != nil && s.capture.ScanDir() != "" {
		scanDir := s.capture.ScanDir()
		appendDir(scanDir)
		if parent := path.Dir(filepath.ToSlash(j.RelPath)); parent != "." && parent != "/" {
			appendDir(filepath.Join(scanDir, filepath.FromSlash(parent)))
		}
	}
	fileExt := firstNonEmptyString(j.FileExt, filepath.Ext(j.FileName))
	candidates := make([]string, 0, 4)
	appendCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		for _, item := range candidates {
			if item == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}
	for _, dir := range dirCandidates {
		candidates = candidates[:0]
		appendCandidate(filepath.Join(dir, j.FileName))
		if fileExt != "" {
			appendCandidate(filepath.Join(dir, j.Number+fileExt))
			appendCandidate(filepath.Join(dir, j.RawNumber+fileExt))
			appendCandidate(filepath.Join(dir, j.CleanedNumber+fileExt))
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				if err := s.syncJobSourcePath(ctx, j, candidate); err != nil {
					return "", err
				}
				return candidate, nil
			}
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		exactMatches := make([]string, 0, 1)
		prefixMatches := make([]string, 0, 2)
		fallbackMatches := make([]string, 0, 2)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if fileExt != "" && !strings.EqualFold(filepath.Ext(name), fileExt) {
				continue
			}
			fullPath := filepath.Join(dir, name)
			fallbackMatches = append(fallbackMatches, fullPath)
			base := strings.TrimSuffix(name, filepath.Ext(name))
			for _, expected := range []string{j.Number, j.RawNumber, j.CleanedNumber} {
				expected = strings.TrimSpace(expected)
				if expected == "" {
					continue
				}
				if strings.EqualFold(base, expected) {
					exactMatches = append(exactMatches, fullPath)
					break
				}
				if strings.HasPrefix(strings.ToLower(base), strings.ToLower(expected)+".") || strings.HasPrefix(strings.ToLower(base), strings.ToLower(expected)+"-") {
					prefixMatches = append(prefixMatches, fullPath)
					break
				}
			}
		}
		if len(exactMatches) == 1 {
			if err := s.syncJobSourcePath(ctx, j, exactMatches[0]); err != nil {
				return "", err
			}
			return exactMatches[0], nil
		}
		if len(prefixMatches) == 1 {
			if err := s.syncJobSourcePath(ctx, j, prefixMatches[0]); err != nil {
				return "", err
			}
			return prefixMatches[0], nil
		}
		if len(fallbackMatches) == 1 {
			if err := s.syncJobSourcePath(ctx, j, fallbackMatches[0]); err != nil {
				return "", err
			}
			return fallbackMatches[0], nil
		}
	}
	return "", fmt.Errorf("job source file not found: %s", j.AbsPath)
}

func (s *Service) syncJobSourcePath(ctx context.Context, j *jobdef.Job, sourcePath string) error {
	if sourcePath == "" || sourcePath == j.AbsPath {
		return nil
	}
	fileName := filepath.Base(sourcePath)
	fileExt := filepath.Ext(fileName)
	relPath := fileName
	if parent := path.Dir(filepath.ToSlash(j.RelPath)); parent != "." && parent != "/" {
		relPath = path.Join(parent, fileName)
	}
	if err := s.jobRepo.UpdateSourcePath(ctx, j.ID, fileName, fileExt, relPath, sourcePath); err != nil {
		return err
	}
	j.FileName = fileName
	j.FileExt = fileExt
	j.RelPath = relPath
	j.AbsPath = sourcePath
	_ = s.logRepo.Add(ctx, j.ID, "warn", "source", "job source path refreshed", sourcePath)
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) GetJobConflict(ctx context.Context, job *jobdef.Job) (*JobConflict, error) {
	if job == nil {
		return nil, nil
	}
	index, err := s.buildConflictIndex(ctx)
	if err != nil {
		return nil, err
	}
	conflict, ok := index[job.ID]
	if !ok {
		return nil, nil
	}
	return &conflict, nil
}

func (s *Service) ApplyJobConflicts(ctx context.Context, jobs []jobdef.Job) error {
	index, err := s.buildConflictIndex(ctx)
	if err != nil {
		return err
	}
	for idx := range jobs {
		if conflict, ok := index[jobs[idx].ID]; ok {
			jobs[idx].ConflictReason = conflict.Reason
			jobs[idx].ConflictTarget = conflict.Target
		} else {
			jobs[idx].ConflictReason = ""
			jobs[idx].ConflictTarget = ""
		}
	}
	return nil
}

func (s *Service) buildConflictIndex(ctx context.Context) (map[int64]JobConflict, error) {
	result, err := s.jobRepo.ListJobs(ctx, nil, "", 1, 0)
	if err != nil {
		return nil, err
	}
	jobs := result.Items
	index := make(map[int64]JobConflict, len(jobs))
	grouped := make(map[string][]jobdef.Job)
	for _, job := range jobs {
		if job.Status == jobdef.StatusDone {
			continue
		}
		key := buildJobConflictKey(&job)
		if key == "" {
			continue
		}
		grouped[key] = append(grouped[key], job)
	}
	for _, items := range grouped {
		if len(items) <= 1 {
			continue
		}
		targets := make([]string, 0, len(items))
		for _, item := range items {
			targets = append(targets, item.RelPath)
		}
		sort.Strings(targets)
		targetText := strings.Join(targets, " | ")
		for _, item := range items {
			index[item.ID] = JobConflict{
				Reason: "存在同目标文件名冲突",
				Target: targetText,
			}
		}
	}
	return index, nil
}

func buildJobConflictKey(job *jobdef.Job) string {
	if job == nil {
		return ""
	}
	numberText := strings.TrimSpace(job.Number)
	if numberText == "" {
		return ""
	}
	parsed, err := number.Parse(numberText)
	base := strings.ToUpper(numberText)
	if err == nil && parsed != nil {
		base = strings.ToUpper(parsed.GenerateFileName())
	}
	ext := strings.ToLower(strings.TrimSpace(firstNonEmptyString(job.FileExt, filepath.Ext(job.FileName))))
	if ext == "" {
		return base
	}
	return base + ext
}

func (s *Service) failJob(ctx context.Context, jobID int64, message string) {
	_, _ = s.jobRepo.UpdateStatus(ctx, jobID, []jobdef.Status{jobdef.StatusProcessing}, jobdef.StatusFailed, message)
	_ = s.logRepo.Add(ctx, jobID, "error", "job", message, "")
}

func (s *Service) finish(jobID int64) {
	s.mu.Lock()
	delete(s.running, jobID)
	s.mu.Unlock()
}

func (s *Service) claim(jobID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[jobID]; ok {
		return false
	}
	s.running[jobID] = struct{}{}
	return true
}
