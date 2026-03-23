package job

import (
	"context"
	"encoding/json"
	"fmt"
	stdimage "image"
	"os"
	"sync"

	"github.com/xxxsen/yamdc/internal/capture"
	imgutil "github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
)

type Service struct {
	jobRepo    *repository.JobRepository
	logRepo    *repository.LogRepository
	scrapeRepo *repository.ScrapeDataRepository
	capture    *capture.Capture

	mu      sync.Mutex
	running map[int64]struct{}
}

func NewService(
	jobRepo *repository.JobRepository,
	logRepo *repository.LogRepository,
	scrapeRepo *repository.ScrapeDataRepository,
	cap *capture.Capture,
) *Service {
	return &Service{
		jobRepo:    jobRepo,
		logRepo:    logRepo,
		scrapeRepo: scrapeRepo,
		capture:    cap,
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

func (s *Service) UpdateNumber(ctx context.Context, jobID int64, input string) (*jobdef.Job, error) {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if j == nil {
		return nil, fmt.Errorf("job not found")
	}
	if j.Status != jobdef.StatusInit && j.Status != jobdef.StatusFailed {
		return nil, fmt.Errorf("job number can only be edited in init or failed status")
	}
	fc, err := s.capture.ResolveFileContext(j.AbsPath, input)
	if err != nil {
		return nil, fmt.Errorf("validate number failed: %w", err)
	}
	numberText := fc.Number.GenerateFileName()
	if err := s.jobRepo.UpdateNumber(ctx, jobID, numberText, "manual", "success", "high", ""); err != nil {
		return nil, err
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "number", "job number updated", numberText)
	return s.jobRepo.GetByID(ctx, jobID)
}

func (s *Service) SaveReviewData(ctx context.Context, jobID int64, reviewData string) error {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
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
		return err
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "review", "review data saved", "")
	return nil
}

func (s *Service) CropPosterFromCover(ctx context.Context, jobID int64, x, y, width, height int) (*model.File, error) {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
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
	raw, err := store.GetData(ctx, meta.Cover.Key)
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
	key, err := store.AnonymousPutData(ctx, croppedRaw)
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
		return nil, err
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "review", "poster cropped from cover", fmt.Sprintf("%d,%d,%d,%d", x, y, width, height))
	return meta.Poster, nil
}

func (s *Service) Import(ctx context.Context, jobID int64) error {
	if !s.claim(jobID) {
		return fmt.Errorf("job is already running")
	}
	defer s.finish(jobID)

	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	if j == nil {
		return fmt.Errorf("job not found")
	}
	if j.Status != jobdef.StatusReviewing {
		return fmt.Errorf("job is not in reviewing status")
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
	fc, err := s.capture.ResolveFileContext(j.AbsPath, j.Number)
	if err != nil {
		return fmt.Errorf("resolve file context failed: %w", err)
	}
	fc.Meta = &meta
	_ = s.logRepo.Add(ctx, jobID, "info", "import", "import started", "")
	if err := s.capture.ImportMeta(ctx, fc); err != nil {
		_ = s.logRepo.Add(ctx, jobID, "error", "import", "import failed", err.Error())
		return err
	}
	if err := s.scrapeRepo.SaveFinalData(ctx, jobID, payload); err != nil {
		return err
	}
	if err := s.jobRepo.MarkDone(ctx, jobID); err != nil {
		return err
	}
	_ = s.logRepo.Add(ctx, jobID, "info", "import", "import completed", fc.SaveDir)
	return nil
}

func (s *Service) Delete(ctx context.Context, jobID int64) error {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
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
		return err
	}
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
	_ = s.logRepo.Add(ctx, jobID, "info", "prepare", "resolve file context", j.AbsPath)
	fc, err := s.capture.ResolveFileContext(j.AbsPath, j.Number)
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
