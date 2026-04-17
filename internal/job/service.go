package job

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
	"go.uber.org/zap"
)

var (
	errJobNotFound             = errors.New("job not found")
	errJobNotReviewing         = errors.New("job is not in reviewing status")
	errJobNumberEditNotAllowed = errors.New("job number can only be edited in init or failed status")
	errScrapeDataNotFound      = errors.New("scrape data not found")
	errCoverNotFound           = errors.New("cover not found")
	errCropRectOutOfBounds     = errors.New("crop rectangle out of bounds")
	errJobAlreadyRunning       = errors.New("job is already running")
	errConflict                = errors.New("job conflict")
	errJobStatusNotDeletable   = errors.New("job status does not allow delete")
	errJobCurrentlyRunning     = errors.New("job is currently running")
	errJobNumberRequiresReview = errors.New("job number requires manual edit before scraping")
	errJobStatusNotRunnable    = errors.New("job status is not runnable")
	errJobSourcePathEmpty      = errors.New("job source path is empty")
	errJobSourceNotFound       = errors.New("job source file not found")
	errNoConflict              = errors.New("no job conflict")
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
	queue   chan queuedJob
	workWG  sync.WaitGroup
}

type queuedJob struct {
	ctx   context.Context
	jobID int64
}

type Conflict struct {
	Reason string
	Target string
}

func NewService(
	jobRepo *repository.JobRepository,
	logRepo *repository.LogRepository,
	scrapeRepo *repository.ScrapeDataRepository,
	capt *capture.Capture,
	storage store.IStorage,
) *Service {
	svc := &Service{
		jobRepo:    jobRepo,
		logRepo:    logRepo,
		scrapeRepo: scrapeRepo,
		capture:    capt,
		storage:    storage,
		running:    make(map[int64]struct{}),
		queue:      make(chan queuedJob, 1024),
	}
	go svc.runWorker()
	return svc
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
	items, err := s.logRepo.ListByJobID(ctx, jobID, 500)
	if err != nil {
		return nil, fmt.Errorf("list job logs: %w", err)
	}
	return items, nil
}

func (s *Service) addJobLog(ctx context.Context, jobID int64, level, stage, message, detail string) {
	if s == nil || s.logRepo == nil {
		return
	}
	_ = s.logRepo.Add(ctx, jobID, level, stage, message, detail)
}

func buildScrapeSummary(fc *model.FileContext) string {
	if fc == nil {
		return ""
	}
	numberID := ""
	if fc.Number != nil {
		numberID = fc.Number.GetNumberID()
	}
	parts := make([]string, 0, 8)
	parts = append(parts,
		fmt.Sprintf("file=%s", strings.TrimSpace(fc.FileName)),
		fmt.Sprintf("number=%s", firstNonEmptyString(numberID, fc.SaveFileBase)),
	)
	if fc.Meta != nil {
		parts = append(parts,
			fmt.Sprintf("meta_number=%s", strings.TrimSpace(fc.Meta.Number)),
			fmt.Sprintf("title=%s", strings.TrimSpace(fc.Meta.Title)),
			fmt.Sprintf("title_translated=%s", strings.TrimSpace(fc.Meta.TitleTranslated)),
			fmt.Sprintf("actors=%d", len(fc.Meta.Actors)),
			fmt.Sprintf("samples=%d", len(fc.Meta.SampleImages)),
			fmt.Sprintf("source=%s", strings.TrimSpace(fc.Meta.ExtInfo.ScrapeInfo.Source)),
		)
		if fc.Meta.Cover != nil {
			parts = append(parts, fmt.Sprintf("cover=%s", strings.TrimSpace(fc.Meta.Cover.Name)))
		}
		if fc.Meta.Poster != nil {
			parts = append(parts, fmt.Sprintf("poster=%s", strings.TrimSpace(fc.Meta.Poster.Name)))
		}
	}
	return strings.Join(parts, "\n")
}

func buildJobFailureDetail(job *jobdef.Job, sourcePath string, fc *model.FileContext, err error) string {
	parts := make([]string, 0, 10)
	if job != nil {
		parts = append(parts,
			fmt.Sprintf("job_id=%d", job.ID),
			fmt.Sprintf("status=%s", job.Status),
			fmt.Sprintf("job_number=%s", strings.TrimSpace(job.Number)),
			fmt.Sprintf("raw_number=%s", strings.TrimSpace(job.RawNumber)),
			fmt.Sprintf("cleaned_number=%s", strings.TrimSpace(job.CleanedNumber)),
			fmt.Sprintf("source_file=%s", strings.TrimSpace(job.AbsPath)),
		)
	}
	if strings.TrimSpace(sourcePath) != "" {
		parts = append(parts, fmt.Sprintf("resolved_source=%s", strings.TrimSpace(sourcePath)))
	}
	if fc != nil {
		parsedNumber := ""
		if fc.Number != nil {
			parsedNumber = fc.Number.GetNumberID()
		}
		parts = append(parts,
			fmt.Sprintf("parsed_number=%s", strings.TrimSpace(parsedNumber)),
			fmt.Sprintf("save_file_base=%s", strings.TrimSpace(fc.SaveFileBase)),
		)
		if fc.Meta != nil {
			parts = append(parts,
				fmt.Sprintf("meta_source=%s", strings.TrimSpace(fc.Meta.ExtInfo.ScrapeInfo.Source)),
				fmt.Sprintf("meta_title=%s", strings.TrimSpace(fc.Meta.Title)),
				fmt.Sprintf("meta_samples=%d", len(fc.Meta.SampleImages)),
			)
		}
	}
	if err != nil {
		parts = append(parts, fmt.Sprintf("error=%v", err))
	}
	return strings.Join(parts, "\n")
}

func (s *Service) GetScrapeData(ctx context.Context, jobID int64) (*repository.ScrapeData, error) {
	data, err := s.scrapeRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get scrape data: %w", err)
	}
	return data, nil
}

func (s *Service) SetImportGuard(fn func(context.Context) error) {
	s.importGuard = fn
}

func (s *Service) UpdateNumber(ctx context.Context, jobID int64, input string) (*jobdef.Job, error) {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID), zap.String("number", strings.TrimSpace(input)))
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before number update failed", zap.Error(err))
		return nil, fmt.Errorf("load job before number update: %w", err)
	}
	if j == nil {
		return nil, errJobNotFound
	}
	if j.Status != jobdef.StatusInit && j.Status != jobdef.StatusFailed {
		return nil, errJobNumberEditNotAllowed
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
		return nil, fmt.Errorf("persist updated job number: %w", err)
	}
	s.addJobLog(ctx, jobID, "info", "number", "job number updated", numberText)
	logger.Info("job number updated", zap.String("normalized_number", numberText))
	updated, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("reload job after number update: %w", err)
	}
	if updated != nil {
		if conflict, err := s.GetConflict(ctx, updated); err == nil && conflict != nil {
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
		return fmt.Errorf("load job before saving review data: %w", err)
	}
	if j == nil {
		return errJobNotFound
	}
	if j.Status != jobdef.StatusReviewing {
		return errJobNotReviewing
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(reviewData), &meta); err != nil {
		return fmt.Errorf("invalid review json: %w", err)
	}
	if err := s.scrapeRepo.SaveReviewData(ctx, jobID, reviewData); err != nil {
		logger.Error("save review data failed", zap.Error(err))
		return fmt.Errorf("save review data: %w", err)
	}
	s.addJobLog(ctx, jobID, "info", "review", "review data saved", "")
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
	meta, err := s.loadReviewingMeta(ctx, logger, jobID)
	if err != nil {
		return nil, err
	}
	if meta.Cover == nil || meta.Cover.Key == "" {
		return nil, errCoverNotFound
	}
	posterKey, err := s.cropAndStorePoster(ctx, meta.Cover.Key, x, y, width, height)
	if err != nil {
		return nil, err
	}
	meta.Poster = &model.File{Name: "./poster.jpg", Key: posterKey}
	reviewData, err := json.Marshal(&meta)
	if err != nil {
		return nil, fmt.Errorf("marshal review meta failed: %w", err)
	}
	if err := s.scrapeRepo.SaveReviewData(ctx, jobID, string(reviewData)); err != nil {
		logger.Error("save cropped poster review data failed", zap.Error(err))
		return nil, fmt.Errorf("save cropped poster review data: %w", err)
	}
	s.addJobLog(ctx, jobID, "info", "review", "poster cropped from cover", fmt.Sprintf("%d,%d,%d,%d", x, y, width, height))
	logger.Info("poster cropped from cover", zap.String("poster_key", meta.Poster.Key))
	return meta.Poster, nil
}

func (s *Service) loadReviewingMeta(ctx context.Context, logger *zap.Logger, jobID int64) (model.MovieMeta, error) {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before review action failed", zap.Error(err))
		return model.MovieMeta{}, fmt.Errorf("load job before review action: %w", err)
	}
	if j == nil {
		return model.MovieMeta{}, errJobNotFound
	}
	if j.Status != jobdef.StatusReviewing {
		return model.MovieMeta{}, errJobNotReviewing
	}
	data, err := s.scrapeRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return model.MovieMeta{}, fmt.Errorf("get scrape data: %w", err)
	}
	if data == nil {
		return model.MovieMeta{}, errScrapeDataNotFound
	}
	payload := data.RawData
	if data.ReviewData != "" {
		payload = data.ReviewData
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(payload), &meta); err != nil {
		return model.MovieMeta{}, fmt.Errorf("parse review meta failed: %w", err)
	}
	return meta, nil
}

func (s *Service) cropAndStorePoster(ctx context.Context, coverKey string, x, y, width, height int) (string, error) {
	raw, err := store.GetDataFrom(ctx, s.storage, coverKey)
	if err != nil {
		return "", fmt.Errorf("load cover failed: %w", err)
	}
	img, err := imgutil.LoadImage(raw)
	if err != nil {
		return "", fmt.Errorf("decode cover failed: %w", err)
	}
	bounds := img.Bounds()
	rect := stdimage.Rect(x, y, x+width, y+height)
	if rect.Min.X < bounds.Min.X || rect.Min.Y < bounds.Min.Y || rect.Max.X > bounds.Max.X || rect.Max.Y > bounds.Max.Y {
		return "", errCropRectOutOfBounds
	}
	cropped, err := imgutil.CutImageViaRectangle(img, rect)
	if err != nil {
		return "", fmt.Errorf("crop poster failed: %w", err)
	}
	croppedRaw, err := imgutil.WriteImageToBytes(cropped)
	if err != nil {
		return "", fmt.Errorf("encode poster failed: %w", err)
	}
	key, err := store.AnonymousPutDataTo(ctx, s.storage, croppedRaw)
	if err != nil {
		return "", fmt.Errorf("store cropped poster: %w", err)
	}
	return key, nil
}

func (s *Service) Import(ctx context.Context, jobID int64) error {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID))
	if !s.claim(jobID) {
		logger.Warn("import skipped because job is already running")
		return errJobAlreadyRunning
	}
	defer s.finish(jobID)

	j, err := s.validateImportPreconditions(ctx, logger, jobID)
	if err != nil {
		return err
	}
	data, err := s.scrapeRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get scrape data for import: %w", err)
	}
	if data == nil {
		return errScrapeDataNotFound
	}
	payload := data.RawData
	if data.ReviewData != "" {
		payload = data.ReviewData
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(payload), &meta); err != nil {
		return fmt.Errorf("parse final meta failed: %w", err)
	}
	return s.performImport(ctx, logger, j, jobID, &meta, payload)
}

func (s *Service) validateImportPreconditions(
	ctx context.Context, logger *zap.Logger, jobID int64,
) (*jobdef.Job, error) {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before import failed", zap.Error(err))
		return nil, fmt.Errorf("load job before import: %w", err)
	}
	if j == nil {
		return nil, errJobNotFound
	}
	if j.Status != jobdef.StatusReviewing {
		return nil, errJobNotReviewing
	}
	if s.importGuard != nil {
		if err := s.importGuard(ctx); err != nil {
			logger.Warn("import blocked by guard", zap.Error(err))
			return nil, err
		}
	}
	conflict, err := s.GetConflict(ctx, j)
	if err != nil && !errors.Is(err, errNoConflict) {
		logger.Error("check job conflict before import failed", zap.Error(err))
		return nil, err
	}
	if conflict != nil {
		logger.Warn("import blocked by conflict",
			zap.String("reason", conflict.Reason),
			zap.String("target", conflict.Target),
		)
		return nil, fmt.Errorf("%s: %s: %w", conflict.Reason, conflict.Target, errConflict)
	}
	return j, nil
}

func (s *Service) performImport(
	ctx context.Context,
	logger *zap.Logger,
	j *jobdef.Job,
	jobID int64,
	meta *model.MovieMeta,
	payload string,
) error {
	sourcePath, err := s.resolveJobSourcePath(ctx, j)
	if err != nil {
		return err
	}
	fc, err := s.capture.ResolveFileContext(sourcePath, j.Number)
	if err != nil {
		return fmt.Errorf("resolve file context failed: %w", err)
	}
	fc.Meta = meta
	s.addJobLog(ctx, jobID, "info", "import", "import started", "")
	logger.Info("import started", zap.String("number", j.Number))
	if err := s.capture.ImportMeta(ctx, fc); err != nil {
		s.addJobLog(ctx, jobID, "error", "import", "import failed", err.Error())
		logger.Error("import failed", zap.Error(err))
		return fmt.Errorf("import meta: %w", err)
	}
	if err := s.scrapeRepo.SaveFinalData(ctx, jobID, payload); err != nil {
		return fmt.Errorf("save final data: %w", err)
	}
	if err := s.jobRepo.MarkDone(ctx, jobID); err != nil {
		return fmt.Errorf("mark job done: %w", err)
	}
	s.addJobLog(ctx, jobID, "info", "import", "import completed", fc.SaveDir)
	logger.Info("import completed", zap.String("save_dir", fc.SaveDir))
	return nil
}

func (s *Service) Delete(ctx context.Context, jobID int64) error {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID))
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load job before delete failed", zap.Error(err))
		return fmt.Errorf("load job before delete: %w", err)
	}
	if j == nil {
		return errJobNotFound
	}
	switch j.Status {
	case jobdef.StatusInit, jobdef.StatusFailed, jobdef.StatusReviewing:
	case jobdef.StatusProcessing, jobdef.StatusDone:
		return errJobStatusNotDeletable
	default:
		return errJobStatusNotDeletable
	}

	if !s.claim(jobID) {
		logger.Warn("delete skipped because job is currently running")
		return errJobCurrentlyRunning
	}
	defer s.finish(jobID)

	if err := os.Remove(j.AbsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete source file failed: %w", err)
	}
	if err := s.scrapeRepo.DeleteByJobID(ctx, jobID); err != nil {
		return fmt.Errorf("delete scrape data: %w", err)
	}
	if err := s.logRepo.DeleteByJobID(ctx, jobID); err != nil {
		return fmt.Errorf("delete job logs: %w", err)
	}
	if err := s.jobRepo.SoftDelete(ctx, jobID); err != nil {
		logger.Error("soft delete job failed", zap.Error(err))
		return fmt.Errorf("soft delete job: %w", err)
	}
	logger.Info("job deleted", zap.String("path", j.AbsPath))
	return nil
}

func (s *Service) Recover(ctx context.Context) error {
	if err := s.jobRepo.RecoverProcessingJobs(ctx); err != nil {
		return fmt.Errorf("recover processing jobs: %w", err)
	}
	return nil
}

func (s *Service) start(ctx context.Context, jobID int64, allowed []jobdef.Status) error {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}
	if j == nil {
		return errJobNotFound
	}
	if requiresManualNumberReview(j) {
		return errJobNumberRequiresReview
	}
	conflict, err := s.GetConflict(ctx, j)
	if err != nil && !errors.Is(err, errNoConflict) {
		return err
	}
	if conflict != nil {
		return fmt.Errorf("%s: %s: %w", conflict.Reason, conflict.Target, errConflict)
	}

	if !s.claim(jobID) {
		return errJobAlreadyRunning
	}

	ok, err := s.jobRepo.UpdateStatus(ctx, jobID, allowed, jobdef.StatusProcessing, "")
	if err != nil {
		s.finish(jobID)
		return fmt.Errorf("update job status: %w", err)
	}
	if !ok {
		s.finish(jobID)
		return errJobStatusNotRunnable
	}
	s.workWG.Add(1)
	s.queue <- queuedJob{
		ctx:   context.WithoutCancel(ctx),
		jobID: jobID,
	}
	return nil
}

func (s *Service) runWorker() {
	for item := range s.queue {
		s.runOne(item.ctx, item.jobID)
		s.workWG.Done()
	}
}

func (s *Service) runOne(ctx context.Context, jobID int64) {
	defer s.finish(jobID)
	s.addJobLog(ctx, jobID, "info", "job", "job started", "")

	j, sourcePath, fc, err := s.prepareJobExecution(ctx, jobID)
	if err != nil {
		return
	}
	s.executeScrapeAndFinalize(ctx, jobID, j, sourcePath, fc)
}

func (s *Service) prepareJobExecution(
	ctx context.Context,
	jobID int64,
) (*jobdef.Job, string, *model.FileContext, error) {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		wrappedErr := fmt.Errorf("load job: %w", err)
		s.failJob(ctx, jobID, "job", fmt.Sprintf("load job failed: %v", err), buildJobFailureDetail(nil, "", nil, err))
		return nil, "", nil, wrappedErr
	}
	if j == nil {
		return nil, "", nil, errJobNotFound
	}
	s.addJobLog(ctx, jobID, "info", "job", "job loaded",
		fmt.Sprintf("status=%s\nnumber=%s\npath=%s", j.Status, j.Number, j.AbsPath))
	sourcePath, err := s.resolveJobSourcePath(ctx, j)
	if err != nil {
		s.failJob(ctx, jobID, "source", err.Error(), buildJobFailureDetail(j, "", nil, err))
		return nil, "", nil, err
	}
	s.addJobLog(ctx, jobID, "info", "source", "job source resolved", sourcePath)
	s.addJobLog(ctx, jobID, "info", "prepare", "resolve file context", sourcePath)
	fc, err := s.capture.ResolveFileContext(sourcePath, j.Number)
	if err != nil {
		wrappedErr := fmt.Errorf("resolve file context: %w", err)
		s.failJob(ctx, jobID, "prepare",
			fmt.Sprintf("resolve file failed: %v", err), buildJobFailureDetail(j, sourcePath, nil, err))
		return nil, "", nil, wrappedErr
	}
	s.addJobLog(ctx, jobID, "info", "prepare", "file context resolved",
		fmt.Sprintf("file=%s\nnumber=%s\nsave_file_base=%s", fc.FileName, fc.Number.GetNumberID(), fc.SaveFileBase))
	return j, sourcePath, fc, nil
}

func (s *Service) executeScrapeAndFinalize(
	ctx context.Context,
	jobID int64,
	j *jobdef.Job,
	sourcePath string,
	fc *model.FileContext,
) {
	s.addJobLog(ctx, jobID, "info", "scrape", "scrape meta start",
		fmt.Sprintf("number=%s\nsource=%s", fc.Number.GetNumberID(), sourcePath))
	if err := s.capture.ScrapeMeta(ctx, fc); err != nil {
		s.failJob(ctx, jobID, "scrape",
			fmt.Sprintf("scrape meta failed: %v", err), buildJobFailureDetail(j, sourcePath, fc, err))
		return
	}
	s.addJobLog(ctx, jobID, "info", "scrape", "scrape meta result", buildScrapeSummary(fc))
	raw, err := json.Marshal(fc.Meta)
	if err != nil {
		s.failJob(ctx, jobID, "scrape",
			fmt.Sprintf("marshal meta failed: %v", err), buildJobFailureDetail(j, sourcePath, fc, err))
		return
	}
	source := ""
	if fc.Meta != nil {
		source = fc.Meta.ExtInfo.ScrapeInfo.Source
	}
	if err := s.scrapeRepo.UpsertRawData(ctx, jobID, source, string(raw)); err != nil {
		s.failJob(ctx, jobID, "scrape",
			fmt.Sprintf("save scrape data failed: %v", err), buildJobFailureDetail(j, sourcePath, fc, err))
		return
	}
	s.addJobLog(ctx, jobID, "info", "scrape", "scrape data saved", fmt.Sprintf("source=%s\nbytes=%d", source, len(raw)))
	s.addJobLog(ctx, jobID, "info", "scrape", "scrape meta completed", source)
	ok, err := s.jobRepo.UpdateStatus(ctx, jobID, []jobdef.Status{jobdef.StatusProcessing}, jobdef.StatusReviewing, "")
	if err != nil {
		s.failJob(ctx, jobID, "job",
			fmt.Sprintf("update reviewing failed: %v", err), buildJobFailureDetail(j, sourcePath, fc, err))
		return
	}
	if !ok {
		s.failJob(ctx, jobID, "job", "job status changed unexpectedly", buildJobFailureDetail(j, sourcePath, fc, nil))
		return
	}
	s.addJobLog(ctx, jobID, "info", "job", "job moved to reviewing", buildScrapeSummary(fc))
}

func (s *Service) resolveJobSourcePath(ctx context.Context, j *jobdef.Job) (string, error) {
	if j == nil {
		return "", errJobNotFound
	}
	if j.AbsPath == "" {
		return "", errJobSourcePathEmpty
	}
	if info, err := os.Stat(j.AbsPath); err == nil && !info.IsDir() {
		return j.AbsPath, nil
	}
	dirCandidates := s.buildDirCandidates(j)
	fileExt := firstNonEmptyString(j.FileExt, filepath.Ext(j.FileName))
	for _, dir := range dirCandidates {
		if found, err := s.matchSourceInDir(ctx, j, dir, fileExt); err != nil {
			return "", err
		} else if found != "" {
			return found, nil
		}
	}
	return "", fmt.Errorf("job source file not found %s: %w", j.AbsPath, errJobSourceNotFound)
}

func (s *Service) buildDirCandidates(j *jobdef.Job) []string {
	dirs := make([]string, 0, 3)
	appendUnique := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		for _, item := range dirs {
			if item == candidate {
				return
			}
		}
		dirs = append(dirs, candidate)
	}
	appendUnique(filepath.Dir(j.AbsPath))
	if s.capture != nil && s.capture.ScanDir() != "" {
		scanDir := s.capture.ScanDir()
		appendUnique(scanDir)
		if parent := path.Dir(filepath.ToSlash(j.RelPath)); parent != "." && parent != "/" {
			appendUnique(filepath.Join(scanDir, filepath.FromSlash(parent)))
		}
	}
	return dirs
}

func (s *Service) matchSourceInDir(ctx context.Context, j *jobdef.Job, dir, fileExt string) (string, error) {
	if found, err := s.matchExplicitCandidates(ctx, j, dir, fileExt); err != nil || found != "" {
		return found, err
	}
	return s.matchByDirScan(ctx, j, dir, fileExt)
}

func (s *Service) matchExplicitCandidates(ctx context.Context, j *jobdef.Job, dir, fileExt string) (string, error) {
	candidates := buildFileCandidates(j, dir, fileExt)
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if err := s.syncJobSourcePath(ctx, j, candidate); err != nil {
				return "", err
			}
			return candidate, nil
		}
	}
	return "", nil
}

func buildFileCandidates(j *jobdef.Job, dir, fileExt string) []string {
	candidates := make([]string, 0, 4)
	appendUnique := func(candidate string) {
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
	appendUnique(filepath.Join(dir, j.FileName))
	if fileExt != "" {
		appendUnique(filepath.Join(dir, j.Number+fileExt))
		appendUnique(filepath.Join(dir, j.RawNumber+fileExt))
		appendUnique(filepath.Join(dir, j.CleanedNumber+fileExt))
	}
	return candidates
}

func (s *Service) matchByDirScan(ctx context.Context, j *jobdef.Job, dir, fileExt string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil //nolint:nilerr // directory not readable is not fatal; skip gracefully
	}
	exact, prefix, fallback := classifyDirEntries(entries, j, dir, fileExt)
	if match := pickSingleMatch(exact, prefix, fallback); match != "" {
		if err := s.syncJobSourcePath(ctx, j, match); err != nil {
			return "", err
		}
		return match, nil
	}
	return "", nil
}

func classifyDirEntries(
	entries []os.DirEntry,
	j *jobdef.Job,
	dir, fileExt string,
) ([]string, []string, []string) {
	exact := make([]string, 0, 1)
	prefix := make([]string, 0, 2)
	fallback := make([]string, 0, 2)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if fileExt != "" && !strings.EqualFold(filepath.Ext(name), fileExt) {
			continue
		}
		fullPath := filepath.Join(dir, name)
		fallback = append(fallback, fullPath)
		base := strings.TrimSuffix(name, filepath.Ext(name))
		classifyEntry(base, fullPath, j, &exact, &prefix)
	}
	return exact, prefix, fallback
}

func classifyEntry(base, fullPath string, j *jobdef.Job, exact, prefix *[]string) {
	for _, expected := range []string{j.Number, j.RawNumber, j.CleanedNumber} {
		expected = strings.TrimSpace(expected)
		if expected == "" {
			continue
		}
		if strings.EqualFold(base, expected) {
			*exact = append(*exact, fullPath)
			return
		}
		lower := strings.ToLower(base)
		expectedLower := strings.ToLower(expected)
		if strings.HasPrefix(lower, expectedLower+".") || strings.HasPrefix(lower, expectedLower+"-") {
			*prefix = append(*prefix, fullPath)
			return
		}
	}
}

func pickSingleMatch(exact, prefix, fallback []string) string {
	if len(exact) == 1 {
		return exact[0]
	}
	if len(prefix) == 1 {
		return prefix[0]
	}
	if len(fallback) == 1 {
		return fallback[0]
	}
	return ""
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
		return fmt.Errorf("update job source path: %w", err)
	}
	j.FileName = fileName
	j.FileExt = fileExt
	j.RelPath = relPath
	j.AbsPath = sourcePath
	s.addJobLog(ctx, j.ID, "warn", "source", "job source path refreshed", sourcePath)
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

func (s *Service) GetConflict(ctx context.Context, job *jobdef.Job) (*Conflict, error) {
	if job == nil {
		return nil, errNoConflict
	}
	if job.Status == jobdef.StatusDone {
		return nil, errNoConflict
	}
	grouped, err := s.loadConflictGroups(ctx, []jobdef.Job{*job})
	if err != nil {
		return nil, err
	}
	key := conflictKeyForJob(job)
	items := grouped[key]
	if len(items) <= 1 {
		return nil, errNoConflict
	}
	return buildConflict(items), nil
}

func (s *Service) ApplyConflicts(ctx context.Context, jobs []jobdef.Job) error {
	grouped, err := s.loadConflictGroups(ctx, jobs)
	if err != nil {
		return err
	}
	for idx := range jobs {
		if jobs[idx].Status == jobdef.StatusDone {
			jobs[idx].ConflictReason = ""
			jobs[idx].ConflictTarget = ""
			continue
		}
		items := grouped[conflictKeyForJob(&jobs[idx])]
		if len(items) <= 1 {
			jobs[idx].ConflictReason = ""
			jobs[idx].ConflictTarget = ""
			continue
		}
		conflict := buildConflict(items)
		jobs[idx].ConflictReason = conflict.Reason
		jobs[idx].ConflictTarget = conflict.Target
	}
	return nil
}

func (s *Service) loadConflictGroups(ctx context.Context, jobs []jobdef.Job) (map[string][]jobdef.Job, error) {
	grouped := make(map[string][]jobdef.Job)
	keys := make([]string, 0, len(jobs))
	for _, job := range jobs {
		if job.Status == jobdef.StatusDone {
			continue
		}
		key := conflictKeyForJob(&job)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return grouped, nil
	}
	items, err := s.jobRepo.ListActiveJobsByConflictKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("list active jobs by conflict keys: %w", err)
	}
	for _, item := range items {
		grouped[item.ConflictKey] = append(grouped[item.ConflictKey], item)
	}
	return grouped, nil
}

func conflictKeyForJob(job *jobdef.Job) string {
	if job == nil {
		return ""
	}
	if strings.TrimSpace(job.ConflictKey) != "" {
		return strings.TrimSpace(job.ConflictKey)
	}
	return jobdef.BuildConflictKey(job.Number, job.FileExt, job.FileName)
}

func buildConflict(items []jobdef.Job) *Conflict {
	if len(items) <= 1 {
		return nil
	}
	targets := make([]string, 0, len(items))
	for _, item := range items {
		targets = append(targets, item.RelPath)
	}
	sort.Strings(targets)
	return &Conflict{
		Reason: "存在同目标文件名冲突",
		Target: strings.Join(targets, " | "),
	}
}

func (s *Service) failJob(ctx context.Context, jobID int64, stage, message, detail string) {
	_, _ = s.jobRepo.UpdateStatus(ctx, jobID, []jobdef.Status{jobdef.StatusProcessing}, jobdef.StatusFailed, message)
	s.addJobLog(ctx, jobID, "error", stage, message, detail)
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

// waitQueuedJobs blocks until all jobs pushed into the internal worker queue
// have been fully processed (including post-status-update DB writes and the
// `finish` cleanup). Safe to call from tests to synchronize async work.
func (s *Service) waitQueuedJobs() {
	s.workWG.Wait()
}
