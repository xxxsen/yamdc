package job

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/capture"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
)

func newTestServiceWithSQLite(t *testing.T) (*Service, *repository.JobRepository) {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())
	return NewService(jobRepo, logRepo, scrapeRepo, nil, store.NewMemStorage()), jobRepo
}

func newTestService(t *testing.T) (*Service, *repository.JobRepository) {
	t.Helper()
	return newTestServiceWithSQLite(t)
}

func insertJob(t *testing.T, repo *repository.JobRepository, absPath string, status jobdef.Status) int64 {
	t.Helper()
	return insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(absPath),
		FileExt:               filepath.Ext(absPath),
		RelPath:               filepath.Base(absPath),
		AbsPath:               absPath,
		Number:                "TEST-001",
		RawNumber:             "TEST001RAW",
		CleanedNumber:         "TEST-001",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, status)
}

func insertJobWithInput(t *testing.T, repo *repository.JobRepository, in repository.UpsertJobInput, status jobdef.Status) int64 {
	t.Helper()
	ctx := context.Background()
	err := repo.UpsertScannedJob(ctx, in)
	require.NoError(t, err)
	items, err := repo.ListJobs(ctx, nil, "", 1, 10)
	require.NoError(t, err)
	require.NotEmpty(t, items.Items)
	id := items.Items[0].ID
	if status != jobdef.StatusInit {
		ok, err := repo.UpdateStatus(ctx, id, []jobdef.Status{jobdef.StatusInit}, status, "")
		require.NoError(t, err)
		require.True(t, ok)
	}
	return id
}

func TestServiceDeleteRejectsProcessing(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "A.mp4"), jobdef.StatusProcessing)

	err := svc.Delete(context.Background(), jobID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not allow delete")
}

func TestServiceDeleteRemovesFileAndSoftDeletesJob(t *testing.T) {
	svc, repo := newTestService(t)
	file := filepath.Join(t.TempDir(), "B.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJob(t, repo, file, jobdef.StatusFailed)

	err := svc.Delete(context.Background(), jobID)
	require.NoError(t, err)

	_, statErr := os.Stat(file)
	require.True(t, os.IsNotExist(statErr))

	got, err := repo.GetByID(context.Background(), jobID)
	require.ErrorIs(t, err, repository.ErrJobNotFound)
	require.Nil(t, got)
}

func TestServiceSaveReviewDataRequiresReviewing(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "C.mp4"), jobdef.StatusInit)

	err := svc.SaveReviewData(context.Background(), jobID, `{"title":"ok"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reviewing")
}

func TestServiceSaveReviewDataRejectsInvalidJSON(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "D.mp4"), jobdef.StatusReviewing)

	err := svc.SaveReviewData(context.Background(), jobID, `{`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid review json")
}

func TestServiceImportRequiresReviewing(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "E.mp4"), jobdef.StatusInit)

	err := svc.Import(context.Background(), jobID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reviewing")
}

func TestServiceRecover(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "F.mp4"), jobdef.StatusProcessing)

	require.NoError(t, svc.Recover(context.Background()))

	got, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, jobdef.StatusFailed, got.Status)
}

func TestServiceRunRequiresManualEditForLowConfidenceNumber(t *testing.T) {
	svc, repo := newTestService(t)
	file := filepath.Join(t.TempDir(), "G.mp4")
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file),
		FileExt:               filepath.Ext(file),
		RelPath:               filepath.Base(file),
		AbsPath:               file,
		Number:                "RAW-NUMBER",
		RawNumber:             "RAW-NUMBER",
		CleanedNumber:         "",
		NumberSource:          "raw",
		NumberCleanStatus:     "no_match",
		NumberCleanConfidence: "low",
		NumberCleanWarnings:   "no candidate matched",
		FileSize:              1,
	}, jobdef.StatusInit)

	err := svc.Run(context.Background(), jobID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires manual edit")

	got, getErr := repo.GetByID(context.Background(), jobID)
	require.NoError(t, getErr)
	require.NotNil(t, got)
	require.Equal(t, jobdef.StatusInit, got.Status)
}

func TestServiceRunProcessesJobsSequentially(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	capt, maxConcurrent := newSequentialTestCapture(t)
	svc.capture = capt

	dir := t.TempDir()
	file1 := filepath.Join(dir, "SEQ-001.mp4")
	file2 := filepath.Join(dir, "SEQ-002.mp4")
	require.NoError(t, os.WriteFile(file1, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(file2, []byte("x"), 0o600))

	jobID1 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file1),
		FileExt:               filepath.Ext(file1),
		RelPath:               filepath.Base(file1),
		AbsPath:               file1,
		Number:                "SEQ-001",
		RawNumber:             "SEQ-001",
		CleanedNumber:         "SEQ-001",
		NumberSource:          "manual",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusInit)
	jobID2 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file2),
		FileExt:               filepath.Ext(file2),
		RelPath:               filepath.Base(file2),
		AbsPath:               file2,
		Number:                "SEQ-002",
		RawNumber:             "SEQ-002",
		CleanedNumber:         "SEQ-002",
		NumberSource:          "manual",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusInit)

	require.NoError(t, svc.Run(context.Background(), jobID1))
	require.NoError(t, svc.Run(context.Background(), jobID2))

	require.Eventually(t, func() bool {
		job1, err := repo.GetByID(context.Background(), jobID1)
		require.NoError(t, err)
		job2, err := repo.GetByID(context.Background(), jobID2)
		require.NoError(t, err)
		return job1 != nil && job2 != nil &&
			job1.Status == jobdef.StatusReviewing &&
			job2.Status == jobdef.StatusReviewing
	}, 5*time.Second, 20*time.Millisecond)

	require.EqualValues(t, 1, maxConcurrent.Load())
}

func TestServiceResolveJobSourcePathFallsBackToRenamedNumberFile(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	newFile := filepath.Join(dir, "HEYZO-0040.mp4")
	require.NoError(t, os.WriteFile(newFile, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "HEYZO-040.mp4",
		FileExt:               ".mp4",
		RelPath:               "HEYZO-040.mp4",
		AbsPath:               filepath.Join(dir, "HEYZO-040.mp4"),
		Number:                "HEYZO-0040",
		RawNumber:             "HEYZO-040",
		CleanedNumber:         "HEYZO-0040",
		NumberSource:          "manual",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusReviewing)

	jobItem, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, jobItem)

	resolved, err := svc.resolveJobSourcePath(context.Background(), jobItem)
	require.NoError(t, err)
	require.Equal(t, newFile, resolved)

	got, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, newFile, got.AbsPath)
	require.Equal(t, "HEYZO-0040.mp4", got.FileName)
	require.Equal(t, "HEYZO-0040.mp4", got.RelPath)
}

type sequentialTestSearcher struct {
	current int32
	max     atomic.Int32
}

type loggingTestSearcher struct {
	meta *model.MovieMeta
	err  error
}

func (s *loggingTestSearcher) Name() string                  { return "logging-test" }
func (s *loggingTestSearcher) Check(_ context.Context) error { return nil }
func (s *loggingTestSearcher) Search(_ context.Context, n *number.Number) (*model.MovieMeta, bool, error) {
	if s.err != nil {
		return nil, false, s.err
	}
	meta := *s.meta
	meta.Number = n.GetNumberID()
	return &meta, true, nil
}

func (s *sequentialTestSearcher) Name() string                  { return "sequential-test" }
func (s *sequentialTestSearcher) Check(_ context.Context) error { return nil }

func (s *sequentialTestSearcher) Search(_ context.Context, n *number.Number) (*model.MovieMeta, bool, error) {
	current := atomic.AddInt32(&s.current, 1)
	defer atomic.AddInt32(&s.current, -1)
	for {
		maxVal := s.max.Load()
		if current <= maxVal || s.max.CompareAndSwap(maxVal, current) {
			break
		}
	}
	time.Sleep(120 * time.Millisecond)
	return &model.MovieMeta{
		Number: n.GetNumberID(),
		Title:  "Sequential Test",
		Cover:  &model.File{Name: "cover.jpg", Key: "cover-key"},
		Poster: &model.File{Name: "poster.jpg", Key: "poster-key"},
	}, true, nil
}

type noopProcessor struct{}

func (p *noopProcessor) Name() string                                          { return "noop" }
func (p *noopProcessor) Process(_ context.Context, _ *model.FileContext) error { return nil }

func newSequentialTestCapture(t *testing.T) (*capture.Capture, *atomic.Int32) {
	t.Helper()
	searcher := &sequentialTestSearcher{}
	capt, err := capture.New(
		capture.WithScanDir(t.TempDir()),
		capture.WithSaveDir(t.TempDir()),
		capture.WithSeacher(searcher),
		capture.WithProcessor(processor.IProcessor(&noopProcessor{})),
		capture.WithStorage(store.NewMemStorage()),
	)
	require.NoError(t, err)
	return capt, &searcher.max
}

func newLoggingTestCapture(t *testing.T, searcher *loggingTestSearcher) *capture.Capture {
	t.Helper()
	capt, err := capture.New(
		capture.WithScanDir(t.TempDir()),
		capture.WithSaveDir(t.TempDir()),
		capture.WithSeacher(searcher),
		capture.WithProcessor(processor.IProcessor(&noopProcessor{})),
		capture.WithStorage(store.NewMemStorage()),
	)
	require.NoError(t, err)
	return capt
}

func findLogByMessage(logs []repository.LogItem, message string) *repository.LogItem {
	for idx := range logs {
		if logs[idx].Message == message {
			return &logs[idx]
		}
	}
	return nil
}

func TestServiceRunOneWritesDetailedScrapeLogs(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "LOG-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file),
		FileExt:               filepath.Ext(file),
		RelPath:               filepath.Base(file),
		AbsPath:               file,
		Number:                "LOG-001",
		RawNumber:             "LOG-001",
		CleanedNumber:         "LOG-001",
		NumberSource:          "manual",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusProcessing)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{
		meta: &model.MovieMeta{
			Title:           "Detailed Title",
			TitleTranslated: "详细标题",
			Actors:          []string{"Alice", "Beth"},
			SampleImages:    []*model.File{{Name: "sample-1.jpg", Key: "sample-1"}},
			Cover:           &model.File{Name: "cover.jpg", Key: "cover-key"},
			Poster:          &model.File{Name: "poster.jpg", Key: "poster-key"},
			ExtInfo: model.ExtInfo{
				ScrapeInfo: model.ScrapeInfo{Source: "test-plugin"},
			},
		},
	})

	svc.runOne(context.Background(), jobID)

	logs, err := svc.ListLogs(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, findLogByMessage(logs, "file context resolved"))

	scrapeResult := findLogByMessage(logs, "scrape meta result")
	require.NotNil(t, scrapeResult)
	require.Contains(t, scrapeResult.Detail, "title=Detailed Title")
	require.Contains(t, scrapeResult.Detail, "title_translated=详细标题")
	require.Contains(t, scrapeResult.Detail, "actors=2")
	require.Contains(t, scrapeResult.Detail, "source=test-plugin")

	saved := findLogByMessage(logs, "scrape data saved")
	require.NotNil(t, saved)
	require.Contains(t, saved.Detail, "source=test-plugin")
	require.Contains(t, saved.Detail, "bytes=")

	moved := findLogByMessage(logs, "job moved to reviewing")
	require.NotNil(t, moved)
	require.Contains(t, moved.Detail, "title=Detailed Title")
}

func TestServiceRunOneWritesDetailedFailureLogs(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "LOG-ERR-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file),
		FileExt:               filepath.Ext(file),
		RelPath:               filepath.Base(file),
		AbsPath:               file,
		Number:                "LOG-ERR-001",
		RawNumber:             "LOG-ERR-001",
		CleanedNumber:         "LOG-ERR-001",
		NumberSource:          "manual",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusProcessing)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{
		err: fmt.Errorf("search backend timeout: upstream 504"),
	})

	svc.runOne(context.Background(), jobID)

	logs, err := svc.ListLogs(context.Background(), jobID)
	require.NoError(t, err)
	failure := findLogByMessage(logs, "scrape meta failed: search number failed, number:LOG-ERR-001, err:search backend timeout: upstream 504")
	require.NotNil(t, failure)
	require.Equal(t, "scrape", failure.Stage)
	require.True(t, strings.Contains(failure.Detail, "job_number=LOG-ERR-001"))
	require.True(t, strings.Contains(failure.Detail, "resolved_source="))
	require.True(t, strings.Contains(failure.Detail, "parsed_number=LOG-ERR-001"))
	require.True(t, strings.Contains(failure.Detail, "error=search number failed, number:LOG-ERR-001, err:search backend timeout: upstream 504"))

	got, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, jobdef.StatusFailed, got.Status)
}

func TestServiceGetConflictDetectsDuplicateTargetFileName(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	jobID1 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "aaa.com@ABC-123.mp4",
		FileExt:               ".mp4",
		RelPath:               "aaa.com@ABC-123.mp4",
		AbsPath:               filepath.Join(dir, "aaa.com@ABC-123.mp4"),
		Number:                "ABC-123",
		RawNumber:             "aaa.com@ABC-123",
		CleanedNumber:         "ABC-123",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)
	jobID2 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "bbb.com@ABC-123.mp4",
		FileExt:               ".mp4",
		RelPath:               "bbb.com@ABC-123.mp4",
		AbsPath:               filepath.Join(dir, "bbb.com@ABC-123.mp4"),
		Number:                "ABC-123",
		RawNumber:             "bbb.com@ABC-123",
		CleanedNumber:         "ABC-123",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)

	job1, err := repo.GetByID(context.Background(), jobID1)
	require.NoError(t, err)
	job2, err := repo.GetByID(context.Background(), jobID2)
	require.NoError(t, err)

	conflict1, err := svc.GetConflict(context.Background(), job1)
	require.NoError(t, err)
	require.NotNil(t, conflict1)
	require.Contains(t, conflict1.Reason, "冲突")

	conflict2, err := svc.GetConflict(context.Background(), job2)
	require.NoError(t, err)
	require.NotNil(t, conflict2)
	require.Contains(t, conflict2.Reason, "冲突")
}

func TestServiceGetConflictAllowsMultiCDTargets(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	jobID1 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "ABC-123-CD1.mp4",
		FileExt:               ".mp4",
		RelPath:               "ABC-123-CD1.mp4",
		AbsPath:               filepath.Join(dir, "ABC-123-CD1.mp4"),
		Number:                "ABC-123-CD1",
		RawNumber:             "ABC-123-CD1",
		CleanedNumber:         "ABC-123-CD1",
		NumberSource:          "raw",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)
	jobID2 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "ABC-123-CD2.mp4",
		FileExt:               ".mp4",
		RelPath:               "ABC-123-CD2.mp4",
		AbsPath:               filepath.Join(dir, "ABC-123-CD2.mp4"),
		Number:                "ABC-123-CD2",
		RawNumber:             "ABC-123-CD2",
		CleanedNumber:         "ABC-123-CD2",
		NumberSource:          "raw",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)

	job1, err := repo.GetByID(context.Background(), jobID1)
	require.NoError(t, err)
	job2, err := repo.GetByID(context.Background(), jobID2)
	require.NoError(t, err)

	conflict1, err := svc.GetConflict(context.Background(), job1)
	require.ErrorIs(t, err, errNoConflict)
	require.Nil(t, conflict1)

	conflict2, err := svc.GetConflict(context.Background(), job2)
	require.ErrorIs(t, err, errNoConflict)
	require.Nil(t, conflict2)
}

func TestServiceGetConflictAllowsSpecialSuffixTargets(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	jobID1 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "ABC-123-C.mp4",
		FileExt:               ".mp4",
		RelPath:               "ABC-123-C.mp4",
		AbsPath:               filepath.Join(dir, "ABC-123-C.mp4"),
		Number:                "ABC-123-C",
		RawNumber:             "ABC-123-C",
		CleanedNumber:         "ABC-123-C",
		NumberSource:          "raw",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)
	jobID2 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "ABC-123-4K.mp4",
		FileExt:               ".mp4",
		RelPath:               "ABC-123-4K.mp4",
		AbsPath:               filepath.Join(dir, "ABC-123-4K.mp4"),
		Number:                "ABC-123-4K",
		RawNumber:             "ABC-123-4K",
		CleanedNumber:         "ABC-123-4K",
		NumberSource:          "raw",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)

	job1, err := repo.GetByID(context.Background(), jobID1)
	require.NoError(t, err)
	job2, err := repo.GetByID(context.Background(), jobID2)
	require.NoError(t, err)

	conflict1, err := svc.GetConflict(context.Background(), job1)
	require.ErrorIs(t, err, errNoConflict)
	require.Nil(t, conflict1)

	conflict2, err := svc.GetConflict(context.Background(), job2)
	require.ErrorIs(t, err, errNoConflict)
	require.Nil(t, conflict2)
}

func TestServiceGetConflictIgnoresExistingSavedirTarget(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "ABC-123.mp4",
		FileExt:               ".mp4",
		RelPath:               "ABC-123.mp4",
		AbsPath:               filepath.Join(dir, "ABC-123.mp4"),
		Number:                "ABC-123",
		RawNumber:             "ABC-123",
		CleanedNumber:         "ABC-123",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)

	jobItem, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, jobItem)

	conflict, err := svc.GetConflict(context.Background(), jobItem)
	require.ErrorIs(t, err, errNoConflict)
	require.Nil(t, conflict)
}

func TestServiceApplyConflictsBatchesCurrentPageKeys(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	dir := t.TempDir()
	jobID1 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "page-a-1@ABC-123.mp4",
		FileExt:               ".mp4",
		RelPath:               "page-a-1@ABC-123.mp4",
		AbsPath:               filepath.Join(dir, "page-a-1@ABC-123.mp4"),
		Number:                "ABC-123",
		RawNumber:             "page-a-1@ABC-123",
		CleanedNumber:         "ABC-123",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)
	_ = insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "page-a-2@ABC-123.mp4",
		FileExt:               ".mp4",
		RelPath:               "page-a-2@ABC-123.mp4",
		AbsPath:               filepath.Join(dir, "page-a-2@ABC-123.mp4"),
		Number:                "ABC-123",
		RawNumber:             "page-a-2@ABC-123",
		CleanedNumber:         "ABC-123",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)
	jobID3 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "page-b-1@XYZ-999.mp4",
		FileExt:               ".mp4",
		RelPath:               "page-b-1@XYZ-999.mp4",
		AbsPath:               filepath.Join(dir, "page-b-1@XYZ-999.mp4"),
		Number:                "XYZ-999",
		RawNumber:             "page-b-1@XYZ-999",
		CleanedNumber:         "XYZ-999",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)
	_ = insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "page-b-2@XYZ-999.mp4",
		FileExt:               ".mp4",
		RelPath:               "page-b-2@XYZ-999.mp4",
		AbsPath:               filepath.Join(dir, "page-b-2@XYZ-999.mp4"),
		Number:                "XYZ-999",
		RawNumber:             "page-b-2@XYZ-999",
		CleanedNumber:         "XYZ-999",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)

	job1, err := repo.GetByID(ctx, jobID1)
	require.NoError(t, err)
	require.NotNil(t, job1)
	job3, err := repo.GetByID(ctx, jobID3)
	require.NoError(t, err)
	require.NotNil(t, job3)

	page := []jobdef.Job{*job1, *job3}
	require.NoError(t, svc.ApplyConflicts(ctx, page))
	require.Contains(t, page[0].ConflictTarget, "page-a-1@ABC-123.mp4")
	require.Contains(t, page[0].ConflictTarget, "page-a-2@ABC-123.mp4")
	require.NotContains(t, page[0].ConflictTarget, "page-b-1@XYZ-999.mp4")
	require.Contains(t, page[1].ConflictTarget, "page-b-1@XYZ-999.mp4")
	require.Contains(t, page[1].ConflictTarget, "page-b-2@XYZ-999.mp4")
	require.NotContains(t, page[1].ConflictTarget, "page-a-1@ABC-123.mp4")
}

func TestServiceApplyConflictsIgnoresDoneAndDeletedJobs(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	dir := t.TempDir()
	jobID1 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "active@ABC-123.mp4",
		FileExt:               ".mp4",
		RelPath:               "active@ABC-123.mp4",
		AbsPath:               filepath.Join(dir, "active@ABC-123.mp4"),
		Number:                "ABC-123",
		RawNumber:             "active@ABC-123",
		CleanedNumber:         "ABC-123",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusInit)
	jobID2 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "done@ABC-123.mp4",
		FileExt:               ".mp4",
		RelPath:               "done@ABC-123.mp4",
		AbsPath:               filepath.Join(dir, "done@ABC-123.mp4"),
		Number:                "ABC-123",
		RawNumber:             "done@ABC-123",
		CleanedNumber:         "ABC-123",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusDone)
	jobID3 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              "deleted@ABC-123.mp4",
		FileExt:               ".mp4",
		RelPath:               "deleted@ABC-123.mp4",
		AbsPath:               filepath.Join(dir, "deleted@ABC-123.mp4"),
		Number:                "ABC-123",
		RawNumber:             "deleted@ABC-123",
		CleanedNumber:         "ABC-123",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}, jobdef.StatusFailed)
	require.NoError(t, repo.SoftDelete(ctx, jobID3))

	job1, err := repo.GetByID(ctx, jobID1)
	require.NoError(t, err)
	require.NotNil(t, job1)

	doneJob := jobdef.Job{
		ID:          jobID2,
		RelPath:     "done@ABC-123.mp4",
		Number:      "ABC-123",
		FileName:    "done@ABC-123.mp4",
		FileExt:     ".mp4",
		Status:      jobdef.StatusDone,
		ConflictKey: "ABC-123.mp4",
	}

	page := []jobdef.Job{*job1, doneJob}
	require.NoError(t, svc.ApplyConflicts(ctx, page))
	require.Equal(t, "", page[0].ConflictReason)
	require.Equal(t, "", page[0].ConflictTarget)
	require.Equal(t, "", page[1].ConflictReason)
	require.Equal(t, "", page[1].ConflictTarget)
}
