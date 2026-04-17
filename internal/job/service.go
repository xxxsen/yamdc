package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
	"go.uber.org/zap"
)

// 跨包共享的 sentinel 错误: review 包等需要通过 errors.Is 判定这些值,
// 在此显式导出以避免两份值漂移。
//
// ErrJobNotFound 直接复用 repository.ErrJobNotFound: jobRepo.GetByID 在 job
// 不存在时返回的正是这个值, 调用方无论写 errors.Is(err, job.ErrJobNotFound)
// 还是 errors.Is(err, repository.ErrJobNotFound) 都能命中, 避免两边 sentinel
// 漂移导致 wrap 之后匹配不上。
var (
	ErrJobNotFound       = repository.ErrJobNotFound
	ErrJobAlreadyRunning = errors.New("job is already running")
	ErrJobConflict       = errors.New("job conflict")
	ErrNoConflict        = errors.New("no job conflict")
)

// job 包内继续用小写别名, 行为/值与导出版本完全一致, 仅命名风格差异。
var (
	errJobNotFound             = ErrJobNotFound
	errJobAlreadyRunning       = ErrJobAlreadyRunning
	errConflict                = ErrJobConflict
	errNoConflict              = ErrNoConflict
	errJobNumberEditNotAllowed = errors.New("job number can only be edited in init or failed status")
	errJobStatusNotDeletable   = errors.New("job status does not allow delete")
	errJobCurrentlyRunning     = errors.New("job is currently running")
	errJobNumberRequiresReview = errors.New("job number requires manual edit before scraping")
	errJobStatusNotRunnable    = errors.New("job status is not runnable")
	errJobSourcePathEmpty      = errors.New("job source path is empty")
	errJobSourceNotFound       = errors.New("job source file not found")
)

type Service struct {
	jobRepo    *repository.JobRepository
	logRepo    *repository.LogRepository
	scrapeRepo *repository.ScrapeDataRepository
	capture    *capture.Capture
	storage    store.IStorage

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

// getBlockingConflict 是 3.2.b 修复点: Import 前校验只阻塞"正在活跃占用"的
// 同 conflict_key 兄弟 job (processing / reviewing), 不阻塞仅处于 init / failed
// 的兄弟 —— 那些 job 没有快照, 不会抢目录。
//
// 与 GetConflict 的区别:
//   - GetConflict 用于 Start/列表展示, 把所有非 done 非 deleted 同 key 兄弟都列为冲突,
//     避免两个 init 同 key 被同时启动;
//   - getBlockingConflict 只关心真正的并发冲突, 允许 A=reviewing+B=init 的场景下
//     A 先 Import 落库 (B 若随后 Start 仍会被 GetConflict/A=done 之后的状态拦住)。
//
// 自己 (job.ID) 永远会从结果集里排除。
func (s *Service) getBlockingConflict(ctx context.Context, job *jobdef.Job) (*Conflict, error) {
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
	active := make([]jobdef.Job, 0, len(items))
	for _, item := range items {
		if item.ID == job.ID {
			continue
		}
		if !isBlockingImportStatus(item.Status) {
			continue
		}
		active = append(active, item)
	}
	if len(active) == 0 {
		return nil, errNoConflict
	}
	active = append(active, *job)
	return buildConflict(active), nil
}

// isBlockingImportStatus 判定一个同 conflict_key 兄弟 job 是否"正在"跟我抢资源。
// 只有 processing / reviewing 算作活跃占用: 前者正在真的抓取, 后者已经产出快照
// 等待用户确认, Import 一旦落盘会和对方目标目录重合。init / failed 没有快照,
// 自然也没有目录占用。done 已经完成, 到 conflict_key 索引不命中此处仅作完整性保留。
//
// default 分支走"保守阻塞": 若未来新增一个未在此处枚举的"活跃"状态,
// 宁可误报也不要静默放行 Import 导致两个 job 同时往同一个目录写 NFO。
// 新增状态时请显式登记到 return false 的 case 以解除阻塞。
func isBlockingImportStatus(status jobdef.Status) bool {
	switch status {
	case jobdef.StatusProcessing, jobdef.StatusReviewing:
		return true
	case jobdef.StatusInit, jobdef.StatusFailed, jobdef.StatusDone:
		return false
	default:
		return true
	}
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
	if _, err := s.jobRepo.UpdateStatus(
		ctx, jobID,
		[]jobdef.Status{jobdef.StatusProcessing}, jobdef.StatusFailed, message,
	); err != nil {
		// 即使状态更新失败, 仍要继续写 job log, 否则排障时看不到失败原因。
		logutil.GetLogger(ctx).Error("fail job: update status to failed failed",
			zap.Int64("job_id", jobID),
			zap.String("stage", stage),
			zap.String("message", message),
			zap.Error(err),
		)
	}
	s.addJobLog(ctx, jobID, "error", stage, message, detail)
}

// Claim / Finish / AddJobLog / ResolveJobSourcePath / GetBlockingConflict 是为 3.2
// 抽出的 internal/review.Service 提供的协作原语。review 包只依赖这些方法与
// jobdef/repository, 不直接反向依赖 job.Service 的内部字段。保持这层边界,
// 未来即便要把 review 包继续拆细也只用看这几个签名。
func (s *Service) Claim(jobID int64) bool {
	return s.claim(jobID)
}

func (s *Service) Finish(jobID int64) {
	s.finish(jobID)
}

func (s *Service) AddJobLog(ctx context.Context, jobID int64, level, stage, message, detail string) {
	s.addJobLog(ctx, jobID, level, stage, message, detail)
}

func (s *Service) ResolveJobSourcePath(ctx context.Context, j *jobdef.Job) (string, error) {
	return s.resolveJobSourcePath(ctx, j)
}

// GetBlockingConflict 用于 Import 前置校验: 只把 processing/reviewing 状态的同
// conflict_key 兄弟算作阻塞方, init/failed 不阻塞。detail 见 getBlockingConflict。
func (s *Service) GetBlockingConflict(ctx context.Context, job *jobdef.Job) (*Conflict, error) {
	return s.getBlockingConflict(ctx, job)
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

// WaitQueuedJobs blocks until all jobs pushed into the internal worker queue
// have been fully processed (including post-status-update DB writes and the
// `finish` cleanup). 目前主要用于测试: 在关闭底层 sqlite / 清理 tempdir 之前
// 同步等待 worker goroutine 完成所有异步写入, 避免 journal 文件残留导致
// "directory not empty" 等 flaky 失败。
//
// 注意 worker 是常驻 goroutine (消费内部 channel, 与单个 ctx 无绑定关系),
// 因此 "cancel 某个 ctx 自动收尾" 的心智模型并不适用。生产侧若要 graceful
// shutdown, 应在确保不会再有新任务入队 (关闭入口 / 停止调用 Run / Import 等)
// 之后调用本方法, 以排空已入队但未处理的任务。
func (s *Service) WaitQueuedJobs() {
	s.workWG.Wait()
}
