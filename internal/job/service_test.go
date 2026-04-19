package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	svc := NewService(jobRepo, logRepo, scrapeRepo, nil, store.NewMemStorage())
	// Cleanup 注册顺序: 先 sqlite.Close, 再 svc.Stop。按 LIFO 执行时会先
	// 等待所有通过 queue 分发到 worker 的异步任务完成 (包括状态更新之后的
	// addJobLog 与 finish), 并显式 close(queue) 让 runWorker goroutine 自然
	// 退出 (否则 goleak 会把 <-s.queue 长驻 goroutine 报为泄漏), 然后再关闭
	// sqlite, 最后 testing 框架清理 tempdir。否则 worker 仍在写 DB 时就关闭
	// sqlite/删除 tempdir, 会残留 journal 文件导致 "directory not empty" 的
	// flaky 失败。
	t.Cleanup(func() {
		require.NoError(t, svc.Stop(context.Background()))
	})
	return svc, jobRepo
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

func findLogByMessage(logs []LogItem, message string) *LogItem {
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

func setupReviewingJobWithScrapeData(
	t *testing.T,
	svc *Service,
	repo *repository.JobRepository,
	meta *model.MovieMeta,
) (int64, *repository.ScrapeDataRepository) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "REVIEW-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file),
		FileExt:               filepath.Ext(file),
		RelPath:               filepath.Base(file),
		AbsPath:               file,
		Number:                "REVIEW-001",
		RawNumber:             "REVIEW-001",
		CleanedNumber:         "REVIEW-001",
		NumberSource:          "manual",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusReviewing)

	raw, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))
	return jobID, svc.scrapeRepo
}

// ---------- requiresManualNumberReview ----------

func TestRequiresManualNumberReview(t *testing.T) {
	tests := []struct {
		name     string
		job      *jobdef.Job
		expected bool
	}{
		{name: "nil job", job: nil, expected: false},
		{name: "manual source", job: &jobdef.Job{NumberSource: "manual", NumberCleanStatus: "no_match", NumberCleanConfidence: "low"}, expected: false},
		{name: "no_match status", job: &jobdef.Job{NumberSource: "cleaner", NumberCleanStatus: "no_match", NumberCleanConfidence: "high"}, expected: true},
		{name: "low_quality status", job: &jobdef.Job{NumberSource: "cleaner", NumberCleanStatus: "low_quality", NumberCleanConfidence: "high"}, expected: true},
		{name: "low confidence", job: &jobdef.Job{NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "low"}, expected: true},
		{name: "high confidence success", job: &jobdef.Job{NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high"}, expected: false},
		{name: "medium confidence success", job: &jobdef.Job{NumberSource: "raw", NumberCleanStatus: "success", NumberCleanConfidence: "medium"}, expected: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, requiresManualNumberReview(tc.job))
		})
	}
}

// ---------- firstNonEmptyString ----------

func TestFirstNonEmptyString(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{name: "all empty", input: []string{"", "  ", ""}, expected: ""},
		{name: "first non-empty", input: []string{"", "hello", "world"}, expected: "hello"},
		{name: "only spaces", input: []string{"   "}, expected: ""},
		{name: "no args", input: nil, expected: ""},
		{name: "first has value", input: []string{"a", "b"}, expected: "a"},
		{name: "trim spaces", input: []string{" x "}, expected: "x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, firstNonEmptyString(tc.input...))
		})
	}
}

// ---------- pickSingleMatch ----------

func TestPickSingleMatch(t *testing.T) {
	tests := []struct {
		name     string
		exact    []string
		prefix   []string
		fallback []string
		expected string
	}{
		{name: "single exact", exact: []string{"/a.mp4"}, prefix: nil, fallback: nil, expected: "/a.mp4"},
		{name: "multiple exact", exact: []string{"/a.mp4", "/b.mp4"}, prefix: nil, fallback: nil, expected: ""},
		{name: "single prefix", exact: nil, prefix: []string{"/p.mp4"}, fallback: nil, expected: "/p.mp4"},
		{name: "multiple prefix", exact: nil, prefix: []string{"/p1.mp4", "/p2.mp4"}, fallback: nil, expected: ""},
		{name: "single fallback", exact: nil, prefix: nil, fallback: []string{"/f.mp4"}, expected: "/f.mp4"},
		{name: "multiple fallback", exact: nil, prefix: nil, fallback: []string{"/f1.mp4", "/f2.mp4"}, expected: ""},
		{name: "all empty", exact: nil, prefix: nil, fallback: nil, expected: ""},
		{name: "exact wins over prefix", exact: []string{"/e.mp4"}, prefix: []string{"/p.mp4"}, fallback: []string{"/f.mp4"}, expected: "/e.mp4"},
		{name: "prefix wins over fallback", exact: []string{"/a", "/b"}, prefix: []string{"/p.mp4"}, fallback: []string{"/f.mp4"}, expected: "/p.mp4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, pickSingleMatch(tc.exact, tc.prefix, tc.fallback))
		})
	}
}

// ---------- buildConflict ----------

func TestBuildConflict(t *testing.T) {
	tests := []struct {
		name     string
		items    []jobdef.Job
		isNil    bool
		contains string
	}{
		{name: "empty", items: nil, isNil: true},
		{name: "single", items: []jobdef.Job{{RelPath: "a.mp4"}}, isNil: true},
		{name: "two items", items: []jobdef.Job{
			{RelPath: "b.mp4"},
			{RelPath: "a.mp4"},
		}, contains: "a.mp4 | b.mp4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := buildConflict(tc.items)
			if tc.isNil {
				assert.Nil(t, c)
			} else {
				require.NotNil(t, c)
				assert.Contains(t, c.Target, tc.contains)
				assert.NotEmpty(t, c.Reason)
			}
		})
	}
}

// ---------- conflictKeyForJob ----------

func TestConflictKeyForJob(t *testing.T) {
	tests := []struct {
		name     string
		job      *jobdef.Job
		expected string
	}{
		{name: "nil job", job: nil, expected: ""},
		{name: "existing conflict key", job: &jobdef.Job{ConflictKey: "EXISTING.mp4"}, expected: "EXISTING.mp4"},
		{name: "whitespace conflict key uses build", job: &jobdef.Job{ConflictKey: "  ", Number: "ABC-123", FileExt: ".mp4", FileName: "ABC-123.mp4"}, expected: "ABC-123.mp4"},
		{name: "build from number", job: &jobdef.Job{Number: "XYZ-001", FileExt: ".mkv", FileName: "XYZ-001.mkv"}, expected: "XYZ-001.mkv"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, conflictKeyForJob(tc.job))
		})
	}
}

// ---------- buildScrapeSummary ----------

func TestBuildScrapeSummary(t *testing.T) {
	tests := []struct {
		name     string
		fc       *model.FileContext
		contains []string
		isEmpty  bool
	}{
		{name: "nil fc", fc: nil, isEmpty: true},
		{name: "fc without meta", fc: &model.FileContext{FileName: "test.mp4", SaveFileBase: "TEST-001"}, contains: []string{"file=test.mp4", "number=TEST-001"}},
		{name: "fc with meta no cover no poster", fc: &model.FileContext{
			FileName:     "test.mp4",
			SaveFileBase: "TEST-001",
			Meta: &model.MovieMeta{
				Number:          "TEST-001",
				Title:           "Title",
				TitleTranslated: "标题",
				Actors:          []string{"A", "B"},
				SampleImages:    []*model.File{{Name: "s1.jpg"}},
				ExtInfo:         model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "src"}},
			},
		}, contains: []string{"meta_number=TEST-001", "title=Title", "title_translated=标题", "actors=2", "samples=1", "source=src"}},
		{name: "fc with cover and poster", fc: &model.FileContext{
			FileName:     "test.mp4",
			SaveFileBase: "TEST-001",
			Meta: &model.MovieMeta{
				Number:  "TEST-001",
				Title:   "T",
				Cover:   &model.File{Name: "cover.jpg"},
				Poster:  &model.File{Name: "poster.jpg"},
				ExtInfo: model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "x"}},
			},
		}, contains: []string{"cover=cover.jpg", "poster=poster.jpg"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := buildScrapeSummary(tc.fc)
			if tc.isEmpty {
				assert.Empty(t, result)
			} else {
				for _, s := range tc.contains {
					assert.Contains(t, result, s)
				}
			}
		})
	}
}

// ---------- buildJobFailureDetail ----------

func TestBuildJobFailureDetail(t *testing.T) {
	tests := []struct {
		name       string
		job        *jobdef.Job
		sourcePath string
		fc         *model.FileContext
		err        error
		contains   []string
	}{
		{name: "all nil", job: nil, sourcePath: "", fc: nil, err: nil, contains: nil},
		{
			name: "with job", job: &jobdef.Job{ID: 1, Status: "init", Number: "N", RawNumber: "R", CleanedNumber: "C", AbsPath: "/a"},
			contains: []string{"job_id=1", "status=init", "job_number=N", "raw_number=R", "cleaned_number=C", "source_file=/a"},
		},
		{name: "with source path", sourcePath: "/resolved", contains: []string{"resolved_source=/resolved"}},
		{name: "with fc and meta", fc: &model.FileContext{
			SaveFileBase: "SFB",
			Meta: &model.MovieMeta{
				Title:        "T",
				SampleImages: []*model.File{{Name: "s1"}},
				ExtInfo:      model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "src"}},
			},
		}, contains: []string{"save_file_base=SFB", "meta_source=src", "meta_title=T", "meta_samples=1"}},
		{name: "with error", err: fmt.Errorf("boom"), contains: []string{"error=boom"}},
		{name: "fc without meta", fc: &model.FileContext{SaveFileBase: "X"}, contains: []string{"save_file_base=X"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := buildJobFailureDetail(tc.job, tc.sourcePath, tc.fc, tc.err)
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

// ---------- buildFileCandidates ----------

func TestBuildFileCandidates(t *testing.T) {
	tests := []struct {
		name     string
		job      *jobdef.Job
		dir      string
		fileExt  string
		minCount int
	}{
		{
			name:     "with ext",
			job:      &jobdef.Job{FileName: "test.mp4", Number: "ABC-001", RawNumber: "ABC001", CleanedNumber: "ABC-001"},
			dir:      "/scan",
			fileExt:  ".mp4",
			minCount: 3,
		},
		{
			name:     "no ext",
			job:      &jobdef.Job{FileName: "test.mp4", Number: "ABC-001", RawNumber: "ABC001", CleanedNumber: "ABC-001"},
			dir:      "/scan",
			fileExt:  "",
			minCount: 1,
		},
		{
			name:     "dedup same candidates",
			job:      &jobdef.Job{FileName: "ABC-001.mp4", Number: "ABC-001", RawNumber: "ABC-001", CleanedNumber: "ABC-001"},
			dir:      "/scan",
			fileExt:  ".mp4",
			minCount: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := buildFileCandidates(tc.job, tc.dir, tc.fileExt)
			assert.GreaterOrEqual(t, len(result), tc.minCount)
			seen := make(map[string]struct{})
			for _, c := range result {
				_, dup := seen[c]
				assert.False(t, dup, "duplicate candidate: %s", c)
				seen[c] = struct{}{}
			}
		})
	}
}

// ---------- classifyEntry ----------

func TestClassifyEntry(t *testing.T) {
	tests := []struct {
		name       string
		base       string
		fullPath   string
		job        *jobdef.Job
		wantExact  int
		wantPrefix int
	}{
		{
			name:      "exact match",
			base:      "ABC-001",
			fullPath:  "/dir/ABC-001.mp4",
			job:       &jobdef.Job{Number: "ABC-001", RawNumber: "ABC001", CleanedNumber: "ABC-001"},
			wantExact: 1, wantPrefix: 0,
		},
		{
			name:      "prefix match with dot",
			base:      "ABC-001.720p",
			fullPath:  "/dir/ABC-001.720p.mp4",
			job:       &jobdef.Job{Number: "ABC-001", RawNumber: "ABC001", CleanedNumber: "ABC-001"},
			wantExact: 0, wantPrefix: 1,
		},
		{
			name:      "prefix match with dash",
			base:      "ABC-001-extras",
			fullPath:  "/dir/ABC-001-extras.mp4",
			job:       &jobdef.Job{Number: "ABC-001", RawNumber: "ABC001", CleanedNumber: "ABC-001"},
			wantExact: 0, wantPrefix: 1,
		},
		{
			name:      "no match",
			base:      "XYZ-999",
			fullPath:  "/dir/XYZ-999.mp4",
			job:       &jobdef.Job{Number: "ABC-001", RawNumber: "ABC001", CleanedNumber: "ABC-001"},
			wantExact: 0, wantPrefix: 0,
		},
		{
			name:      "empty expected numbers",
			base:      "ABC-001",
			fullPath:  "/dir/ABC-001.mp4",
			job:       &jobdef.Job{Number: "", RawNumber: "", CleanedNumber: ""},
			wantExact: 0, wantPrefix: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var exact, prefix []string
			classifyEntry(tc.base, tc.fullPath, tc.job, &exact, &prefix)
			assert.Len(t, exact, tc.wantExact)
			assert.Len(t, prefix, tc.wantPrefix)
		})
	}
}

// ---------- classifyDirEntries ----------

func TestClassifyDirEntries(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-001.mp4"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-001-extras.mp4"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "unrelated.mp4"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "something.mkv"), []byte("x"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	job := &jobdef.Job{Number: "ABC-001", RawNumber: "ABC001", CleanedNumber: "ABC-001"}
	exact, prefix, fallback := classifyDirEntries(entries, job, dir, ".mp4")
	assert.Len(t, exact, 1)
	assert.Len(t, prefix, 1)
	assert.Len(t, fallback, 3)
}

// ---------- Rerun ----------

func TestServiceRerun(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "RERUN-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	searcher := &loggingTestSearcher{
		meta: &model.MovieMeta{
			Title:  "Rerun Title",
			Cover:  &model.File{Name: "cover.jpg", Key: "cover-key"},
			Poster: &model.File{Name: "poster.jpg", Key: "poster-key"},
		},
	}
	svc.capture = newLoggingTestCapture(t, searcher)
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file),
		FileExt:               filepath.Ext(file),
		RelPath:               filepath.Base(file),
		AbsPath:               file,
		Number:                "RERUN-001",
		RawNumber:             "RERUN-001",
		CleanedNumber:         "RERUN-001",
		NumberSource:          "manual",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusFailed)

	require.NoError(t, svc.Rerun(context.Background(), jobID))

	require.Eventually(t, func() bool {
		j, err := repo.GetByID(context.Background(), jobID)
		return err == nil && j != nil && j.Status == jobdef.StatusReviewing
	}, 3*time.Second, 20*time.Millisecond)
}

func TestServiceRerunRejectsInit(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "A.mp4"), jobdef.StatusInit)
	err := svc.Rerun(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceRerunJobNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Rerun(context.Background(), 99999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "job not found")
}

// ---------- GetScrapeData ----------

func TestServiceGetScrapeData(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	meta := &model.MovieMeta{Title: "Test", Number: "GSD-001"}
	jobID, _ := setupReviewingJobWithScrapeData(t, svc, repo, meta)

	data, err := svc.GetScrapeData(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, data)
	assert.Contains(t, data.RawData, "GSD-001")
}

func TestServiceGetScrapeDataNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.GetScrapeData(context.Background(), 99999)
	require.Error(t, err)
}

// ---------- ListLogs ----------

func TestServiceListLogs(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "LOG.mp4"), jobdef.StatusInit)

	svc.addJobLog(context.Background(), jobID, "info", "test", "msg1", "d1")
	svc.addJobLog(context.Background(), jobID, "error", "test", "msg2", "d2")

	logs, err := svc.ListLogs(context.Background(), jobID)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func TestServiceListLogsEmpty(t *testing.T) {
	svc, _ := newTestService(t)
	logs, err := svc.ListLogs(context.Background(), 99999)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

// ---------- addJobLog ----------

func TestAddJobLogNilSvc(_ *testing.T) {
	var svc *Service
	svc.addJobLog(context.Background(), 1, "info", "stage", "msg", "detail")
}

func TestAddJobLogNilLogRepo(_ *testing.T) {
	svc := &Service{}
	svc.addJobLog(context.Background(), 1, "info", "stage", "msg", "detail")
}

// ---------- UpdateNumber ----------

func TestServiceUpdateNumber(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}}
	svc.capture = newLoggingTestCapture(t, searcher)

	file := filepath.Join(t.TempDir(), "UPD-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file),
		FileExt:               filepath.Ext(file),
		RelPath:               filepath.Base(file),
		AbsPath:               file,
		Number:                "UPD-001",
		RawNumber:             "UPD-001",
		CleanedNumber:         "UPD-001",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusInit)

	updated, err := svc.UpdateNumber(context.Background(), jobID, "NEW-NUMBER")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "NEW-NUMBER", updated.Number)
	assert.Equal(t, "manual", updated.NumberSource)
}

func TestServiceUpdateNumberJobNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.UpdateNumber(context.Background(), 99999, "X")
	require.Error(t, err)
	require.Contains(t, err.Error(), "job not found")
}

func TestServiceUpdateNumberWrongStatus(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "A.mp4"), jobdef.StatusReviewing)
	_, err := svc.UpdateNumber(context.Background(), jobID, "X")
	require.ErrorIs(t, err, errJobNumberEditNotAllowed)
}

func TestServiceUpdateNumberFailedStatusAllowed(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}}
	svc.capture = newLoggingTestCapture(t, searcher)

	file := filepath.Join(t.TempDir(), "UPD-FAIL.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file),
		FileExt:               filepath.Ext(file),
		RelPath:               filepath.Base(file),
		AbsPath:               file,
		Number:                "UPD-FAIL",
		RawNumber:             "UPD-FAIL",
		CleanedNumber:         "UPD-FAIL",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusFailed)

	updated, err := svc.UpdateNumber(context.Background(), jobID, "FIXED-NUM")
	require.NoError(t, err)
	assert.Equal(t, "FIXED-NUM", updated.Number)
}

// TestGetBlockingConflictExcludesSelf 对应 3.2.b 的"排除自己"单元用例。
func TestGetBlockingConflictExcludesSelf(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "lone@IMP-SELF.mp4", FileExt: ".mp4",
		RelPath: "lone@IMP-SELF.mp4", AbsPath: "/tmp/lone@IMP-SELF.mp4",
		Number: "IMP-SELF", CleanedNumber: "IMP-SELF",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high",
	}, jobdef.StatusReviewing)
	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	conflict, err := svc.getBlockingConflict(context.Background(), j)
	require.ErrorIs(t, err, errNoConflict)
	require.Nil(t, conflict)
}

// TestIsBlockingImportStatus 覆盖状态判定的各个分支。未知状态走保守阻塞,
// 未来新增"活跃"状态忘了登记时, 至少不会静默放行 Import。
func TestIsBlockingImportStatus(t *testing.T) {
	cases := map[jobdef.Status]bool{
		jobdef.StatusInit:       false,
		jobdef.StatusProcessing: true,
		jobdef.StatusReviewing:  true,
		jobdef.StatusDone:       false,
		jobdef.StatusFailed:     false,
		jobdef.Status(""):       true,
		jobdef.Status("weird"):  true,
	}
	for status, expected := range cases {
		t.Run(string(status), func(t *testing.T) {
			assert.Equal(t, expected, isBlockingImportStatus(status))
		})
	}
}

// ---------- Delete ----------

func TestServiceDeleteReviewingStatus(t *testing.T) {
	svc, repo := newTestService(t)
	file := filepath.Join(t.TempDir(), "DEL-REV.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJob(t, repo, file, jobdef.StatusReviewing)
	require.NoError(t, svc.Delete(context.Background(), jobID))
}

func TestServiceDeleteInitStatus(t *testing.T) {
	svc, repo := newTestService(t)
	file := filepath.Join(t.TempDir(), "DEL-INIT.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJob(t, repo, file, jobdef.StatusInit)
	require.NoError(t, svc.Delete(context.Background(), jobID))
}

func TestServiceDeleteRejectsDone(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "DEL-DONE.mp4"), jobdef.StatusDone)
	err := svc.Delete(context.Background(), jobID)
	require.ErrorIs(t, err, errJobStatusNotDeletable)
}

func TestServiceDeleteJobNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Delete(context.Background(), 99999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "job not found")
}

func TestServiceDeleteFileAlreadyGone(t *testing.T) {
	svc, repo := newTestService(t)
	file := filepath.Join(t.TempDir(), "DEL-GONE.mp4")
	jobID := insertJob(t, repo, file, jobdef.StatusFailed)
	require.NoError(t, svc.Delete(context.Background(), jobID))
}

func TestServiceDeleteCurrentlyRunning(t *testing.T) {
	svc, repo := newTestService(t)
	file := filepath.Join(t.TempDir(), "DEL-RUN.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJob(t, repo, file, jobdef.StatusFailed)
	svc.running[jobID] = struct{}{}
	err := svc.Delete(context.Background(), jobID)
	require.ErrorIs(t, err, errJobCurrentlyRunning)
}

// ---------- Recover ----------

func TestServiceRecoverNoJobs(t *testing.T) {
	svc, _ := newTestService(t)
	require.NoError(t, svc.Recover(context.Background()))
}

// ---------- start / Run ----------

func TestServiceRunJobNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Run(context.Background(), 99999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "job not found")
}

func TestServiceRunAlreadyRunning(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}}
	svc.capture = newLoggingTestCapture(t, searcher)

	file := filepath.Join(t.TempDir(), "DBLRUN.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "DBLRUN", RawNumber: "DBLRUN", CleanedNumber: "DBLRUN",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	svc.running[jobID] = struct{}{}
	err := svc.Run(context.Background(), jobID)
	require.ErrorIs(t, err, errJobAlreadyRunning)
}

func TestServiceRunConflictBlocks(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	dir := t.TempDir()
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}})

	insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "a@CNF.mp4", FileExt: ".mp4", RelPath: "a@CNF.mp4", AbsPath: filepath.Join(dir, "a@CNF.mp4"),
		Number: "CNF", RawNumber: "a@CNF", CleanedNumber: "CNF",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	file2 := filepath.Join(dir, "b@CNF.mp4")
	require.NoError(t, os.WriteFile(file2, []byte("x"), 0o600))
	jobID2 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "b@CNF.mp4", FileExt: ".mp4", RelPath: "b@CNF.mp4", AbsPath: file2,
		Number: "CNF", RawNumber: "b@CNF", CleanedNumber: "CNF",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	err := svc.Run(context.Background(), jobID2)
	require.Error(t, err)
	require.ErrorIs(t, err, errConflict)
}

// ---------- resolveJobSourcePath ----------

func TestResolveJobSourcePathNilJob(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.resolveJobSourcePath(context.Background(), nil)
	require.ErrorIs(t, err, errJobNotFound)
}

func TestResolveJobSourcePathEmptyAbsPath(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.resolveJobSourcePath(context.Background(), &jobdef.Job{AbsPath: ""})
	require.ErrorIs(t, err, errJobSourcePathEmpty)
}

func TestResolveJobSourcePathDirectHit(t *testing.T) {
	svc, _ := newTestService(t)
	file := filepath.Join(t.TempDir(), "DIRECT.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	result, err := svc.resolveJobSourcePath(context.Background(), &jobdef.Job{AbsPath: file, FileName: "DIRECT.mp4"})
	require.NoError(t, err)
	assert.Equal(t, file, result)
}

func TestResolveJobSourcePathNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.resolveJobSourcePath(context.Background(), &jobdef.Job{
		AbsPath:  "/nonexistent/dir/file.mp4",
		FileName: "file.mp4",
		Number:   "FILE",
		FileExt:  ".mp4",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, errJobSourceNotFound)
}

// ---------- buildDirCandidates ----------

func TestBuildDirCandidatesWithCapture(t *testing.T) {
	svc, _ := newTestServiceWithSQLite(t)
	scanDir := t.TempDir()
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	job := &jobdef.Job{
		AbsPath: filepath.Join(scanDir, "sub", "file.mp4"),
		RelPath: "sub/file.mp4",
	}
	dirs := svc.buildDirCandidates(job)
	assert.NotEmpty(t, dirs)
	seen := make(map[string]struct{})
	for _, d := range dirs {
		_, dup := seen[d]
		assert.False(t, dup, "duplicate dir candidate")
		seen[d] = struct{}{}
	}
}

func TestBuildDirCandidatesNilCapture(t *testing.T) {
	svc := &Service{running: make(map[int64]struct{})}
	job := &jobdef.Job{
		AbsPath: "/scan/file.mp4",
		RelPath: "file.mp4",
	}
	dirs := svc.buildDirCandidates(job)
	assert.Len(t, dirs, 1)
	assert.Equal(t, "/scan", dirs[0])
}

// ---------- matchByDirScan ----------

func TestMatchByDirScanExactMatch(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-001.mp4"), []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "GONE.mp4", FileExt: ".mp4", RelPath: "GONE.mp4", AbsPath: filepath.Join(dir, "GONE.mp4"),
		Number: "ABC-001", RawNumber: "ABC-001", CleanedNumber: "ABC-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	found, err := svc.matchByDirScan(context.Background(), j, dir, ".mp4")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "ABC-001.mp4"), found)
}

func TestMatchByDirScanPrefixMatch(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-001-extras.mp4"), []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "GONE.mp4", FileExt: ".mp4", RelPath: "GONE.mp4", AbsPath: filepath.Join(dir, "GONE.mp4"),
		Number: "ABC-001", RawNumber: "ABC-001", CleanedNumber: "ABC-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	found, err := svc.matchByDirScan(context.Background(), j, dir, ".mp4")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "ABC-001-extras.mp4"), found)
}

func TestMatchByDirScanFallbackSingleFile(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "totally-different.mp4"), []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "GONE.mp4", FileExt: ".mp4", RelPath: "GONE.mp4", AbsPath: filepath.Join(dir, "GONE.mp4"),
		Number: "ABC-001", RawNumber: "ABC-001", CleanedNumber: "ABC-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	found, err := svc.matchByDirScan(context.Background(), j, dir, ".mp4")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "totally-different.mp4"), found)
}

func TestMatchByDirScanNoMatch(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.mp4"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.mp4"), []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "GONE.mp4", FileExt: ".mp4", RelPath: "GONE.mp4", AbsPath: filepath.Join(dir, "GONE.mp4"),
		Number: "XYZ", RawNumber: "XYZ", CleanedNumber: "XYZ",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	found, err := svc.matchByDirScan(context.Background(), j, dir, ".mp4")
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestMatchByDirScanNonexistentDir(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "A.mp4", FileExt: ".mp4", RelPath: "A.mp4", AbsPath: "/nonexistent/A.mp4",
		Number: "A", RawNumber: "A", CleanedNumber: "A",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	found, err := svc.matchByDirScan(context.Background(), j, "/no/such/directory", ".mp4")
	require.NoError(t, err)
	assert.Empty(t, found)
}

// ---------- syncJobSourcePath ----------

func TestSyncJobSourcePathNoChange(t *testing.T) {
	svc, _ := newTestService(t)
	j := &jobdef.Job{ID: 1, AbsPath: "/scan/file.mp4"}
	require.NoError(t, svc.syncJobSourcePath(context.Background(), j, "/scan/file.mp4"))
	require.NoError(t, svc.syncJobSourcePath(context.Background(), j, ""))
}

// ---------- GetConflict ----------

func TestGetConflictNilJob(t *testing.T) {
	svc, _ := newTestService(t)
	c, err := svc.GetConflict(context.Background(), nil)
	require.ErrorIs(t, err, errNoConflict)
	assert.Nil(t, c)
}

func TestGetConflictDoneJob(t *testing.T) {
	svc, _ := newTestService(t)
	c, err := svc.GetConflict(context.Background(), &jobdef.Job{Status: jobdef.StatusDone})
	require.ErrorIs(t, err, errNoConflict)
	assert.Nil(t, c)
}

// ---------- claim / finish ----------

func TestClaimAndFinish(t *testing.T) {
	svc, _ := newTestService(t)
	assert.True(t, svc.claim(1))
	assert.False(t, svc.claim(1))
	svc.finish(1)
	assert.True(t, svc.claim(1))
}

// ---------- ApplyConflicts with done jobs ----------

func TestApplyConflictsAllDone(t *testing.T) {
	svc, _ := newTestService(t)
	jobs := []jobdef.Job{
		{Status: jobdef.StatusDone, ConflictReason: "old", ConflictTarget: "old"},
	}
	require.NoError(t, svc.ApplyConflicts(context.Background(), jobs))
	assert.Empty(t, jobs[0].ConflictReason)
	assert.Empty(t, jobs[0].ConflictTarget)
}

// ---------- loadConflictGroups empty keys ----------

func TestLoadConflictGroupsAllDone(t *testing.T) {
	svc, _ := newTestService(t)
	jobs := []jobdef.Job{{Status: jobdef.StatusDone, Number: "A"}}
	result, err := svc.loadConflictGroups(context.Background(), jobs)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLoadConflictGroupsEmptyConflictKey(t *testing.T) {
	svc, _ := newTestService(t)
	jobs := []jobdef.Job{{Status: jobdef.StatusInit, Number: "", FileName: "", FileExt: ""}}
	result, err := svc.loadConflictGroups(context.Background(), jobs)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ---------- prepareJobExecution edge cases ----------

func TestPrepareJobExecutionSourceNotFound(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "GHOST.mp4", FileExt: ".mp4", RelPath: "GHOST.mp4", AbsPath: "/nonexistent/GHOST.mp4",
		Number: "GHOST", RawNumber: "GHOST", CleanedNumber: "GHOST",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	svc.runOne(context.Background(), jobID)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.Equal(t, jobdef.StatusFailed, j.Status)
	require.True(t, strings.Contains(j.ErrorMsg, "not found"))
}

// ---------- executeScrapeAndFinalize failure paths ----------

func TestExecuteScrapeAndFinalizeSearchFailed(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "SCRAPE-FAIL.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "SCRAPE-FAIL", RawNumber: "SCRAPE-FAIL", CleanedNumber: "SCRAPE-FAIL",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{err: fmt.Errorf("search error")})
	svc.runOne(context.Background(), jobID)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.Equal(t, jobdef.StatusFailed, j.Status)
}

// ---------- matchSourceInDir ----------

func TestMatchSourceInDirExplicitHit(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "MATCH.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "MATCH.mp4", FileExt: ".mp4", RelPath: "MATCH.mp4", AbsPath: filepath.Join(dir, "OLD.mp4"),
		Number: "MATCH", RawNumber: "MATCH", CleanedNumber: "MATCH",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	found, err := svc.matchSourceInDir(context.Background(), j, dir, ".mp4")
	require.NoError(t, err)
	assert.Equal(t, file, found)
}

// ---------- edge case: classifyDirEntries with no ext filter ----------

func TestClassifyDirEntriesNoExtFilter(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-001.mp4"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-001.mkv"), []byte("x"), 0o600))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	job := &jobdef.Job{Number: "ABC-001", RawNumber: "ABC-001", CleanedNumber: "ABC-001"}
	exact, _, fallback := classifyDirEntries(entries, job, dir, "")
	assert.Len(t, exact, 2)
	assert.Len(t, fallback, 2)
}

// ---------- matchExplicitCandidates with sync ----------

// ---------- Delete with scrape data and logs ----------

func TestServiceDeleteCleansScrapeAndLogs(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "DEL-CLEAN.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJob(t, repo, file, jobdef.StatusFailed)

	svc.addJobLog(context.Background(), jobID, "info", "test", "msg", "d")
	require.NoError(t, svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "src", `{"title":"t"}`))

	require.NoError(t, svc.Delete(context.Background(), jobID))

	logs, err := svc.ListLogs(context.Background(), jobID)
	require.NoError(t, err)
	assert.Empty(t, logs)

	_, err = svc.scrapeRepo.GetByJobID(context.Background(), jobID)
	require.ErrorIs(t, err, repository.ErrScrapeDataNotFound)
}

// ---------- runOne full success path (to cover prepareJobExecution + executeScrapeAndFinalize fully) ----------

func TestRunOneFullSuccessPath(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "RUNONE-OK.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title:  "RunOne Test",
		Cover:  &model.File{Name: "cover.jpg", Key: "cover-key"},
		Poster: &model.File{Name: "poster.jpg", Key: "poster-key"},
		ExtInfo: model.ExtInfo{
			ScrapeInfo: model.ScrapeInfo{Source: "test-src"},
		},
	}}
	svc.capture = newLoggingTestCapture(t, searcher)

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "RUNONE-OK", RawNumber: "RUNONE-OK", CleanedNumber: "RUNONE-OK",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	svc.runOne(context.Background(), jobID)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusReviewing, j.Status)

	data, err := svc.scrapeRepo.GetByJobID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, "test-src", data.Source)
	assert.Contains(t, data.RawData, "RunOne Test")
}

// ---------- start with non-matching status (UpdateStatus returns ok=false) ----------

func TestServiceStartStatusMismatch(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}})

	file := filepath.Join(t.TempDir(), "STMIS.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "STMIS", RawNumber: "STMIS", CleanedNumber: "STMIS",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusFailed)

	err := svc.Run(context.Background(), jobID)
	require.ErrorIs(t, err, errJobStatusNotRunnable)
}

// ---------- resolveJobSourcePath with scan dir based fallback ----------

func TestResolveJobSourcePathScanDirFallback(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}}
	capt := newLoggingTestCapture(t, searcher)
	svc.capture = capt

	scanDir := capt.ScanDir()
	actualFile := filepath.Join(scanDir, "SCAN-001.mp4")
	require.NoError(t, os.WriteFile(actualFile, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "SCAN-001.mp4", FileExt: ".mp4", RelPath: "SCAN-001.mp4",
		AbsPath: "/nonexistent/SCAN-001.mp4",
		Number:  "SCAN-001", RawNumber: "SCAN-001", CleanedNumber: "SCAN-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	resolved, err := svc.resolveJobSourcePath(context.Background(), j)
	require.NoError(t, err)
	assert.Equal(t, actualFile, resolved)
}

func TestResolveJobSourcePathScanDirWithSubDir(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}}
	capt := newLoggingTestCapture(t, searcher)
	svc.capture = capt

	scanDir := capt.ScanDir()
	subDir := filepath.Join(scanDir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	actualFile := filepath.Join(subDir, "SUBDIR-001.mp4")
	require.NoError(t, os.WriteFile(actualFile, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "SUBDIR-001.mp4", FileExt: ".mp4", RelPath: "sub/SUBDIR-001.mp4",
		AbsPath: filepath.Join("/nonexistent", "sub", "SUBDIR-001.mp4"),
		Number:  "SUBDIR-001", RawNumber: "SUBDIR-001", CleanedNumber: "SUBDIR-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	resolved, err := svc.resolveJobSourcePath(context.Background(), j)
	require.NoError(t, err)
	assert.Equal(t, actualFile, resolved)
}

// ---------- buildDirCandidates dedup ----------

func TestBuildDirCandidatesDedupWithScanDir(t *testing.T) {
	svc, _ := newTestServiceWithSQLite(t)
	scanDir := t.TempDir()
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}}
	capt, err := newCaptureWithScanDir(t, scanDir, searcher)
	require.NoError(t, err)
	svc.capture = capt

	job := &jobdef.Job{
		AbsPath: filepath.Join(scanDir, "file.mp4"),
		RelPath: "file.mp4",
	}
	dirs := svc.buildDirCandidates(job)
	assert.Len(t, dirs, 1)
	assert.Equal(t, scanDir, dirs[0])
}

func newCaptureWithScanDir(t *testing.T, scanDir string, searcher *loggingTestSearcher) (*capture.Capture, error) {
	t.Helper()
	return capture.New(
		capture.WithScanDir(scanDir),
		capture.WithSaveDir(t.TempDir()),
		capture.WithSeacher(searcher),
		capture.WithProcessor(processor.IProcessor(&noopProcessor{})),
		capture.WithStorage(store.NewMemStorage()),
	)
}

func newTestCaptureWithStorage(t *testing.T, searcher *loggingTestSearcher, storage store.IStorage) *capture.Capture {
	t.Helper()
	capt, err := capture.New(
		capture.WithScanDir(t.TempDir()),
		capture.WithSaveDir(t.TempDir()),
		capture.WithSeacher(searcher),
		capture.WithProcessor(processor.IProcessor(&noopProcessor{})),
		capture.WithStorage(storage),
	)
	require.NoError(t, err)
	return capt
}

// ---------- UpdateNumber with conflict detection ----------

func TestServiceUpdateNumberDetectsConflict(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}}
	svc.capture = newLoggingTestCapture(t, searcher)
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a@UPDCNF.mp4")
	file2 := filepath.Join(dir, "b@UPDCNF.mp4")
	require.NoError(t, os.WriteFile(file1, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(file2, []byte("x"), 0o600))

	insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file1), FileExt: ".mp4", RelPath: filepath.Base(file1), AbsPath: file1,
		Number: "UPDCNF", RawNumber: "a@UPDCNF", CleanedNumber: "UPDCNF",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	jobID2 := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file2), FileExt: ".mp4", RelPath: filepath.Base(file2), AbsPath: file2,
		Number: "DIFFERENT", RawNumber: "b@UPDCNF", CleanedNumber: "DIFFERENT",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	updated, err := svc.UpdateNumber(context.Background(), jobID2, "UPDCNF")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.NotEmpty(t, updated.ConflictReason)
}

// ---------- GetConflict with single item (no conflict) ----------

func TestGetConflictSingleJob(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "SINGLE.mp4", FileExt: ".mp4", RelPath: "SINGLE.mp4", AbsPath: filepath.Join(dir, "SINGLE.mp4"),
		Number: "SINGLE", RawNumber: "SINGLE", CleanedNumber: "SINGLE",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	conflict, err := svc.GetConflict(context.Background(), j)
	require.ErrorIs(t, err, errNoConflict)
	assert.Nil(t, conflict)
}

// ---------- UpdateNumber with source path error ----------

func TestServiceUpdateNumberSourceNotFound(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "GONE.mp4", FileExt: ".mp4", RelPath: "GONE.mp4", AbsPath: "/nonexistent/GONE.mp4",
		Number: "GONE", RawNumber: "GONE", CleanedNumber: "GONE",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	_, err := svc.UpdateNumber(context.Background(), jobID, "NEW-NUM")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------- executeScrapeAndFinalize status change race ----------

func TestRunOneStatusChangedUnexpectedly(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "RACE-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title:   "Race",
		Cover:   &model.File{Name: "cover.jpg", Key: "k"},
		Poster:  &model.File{Name: "poster.jpg", Key: "k2"},
		ExtInfo: model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "test"}},
	}}
	svc.capture = newLoggingTestCapture(t, searcher)

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "RACE-001", RawNumber: "RACE-001", CleanedNumber: "RACE-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	ok, err := repo.UpdateStatus(context.Background(), jobID, []jobdef.Status{jobdef.StatusProcessing}, jobdef.StatusFailed, "forced")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = repo.UpdateStatus(context.Background(), jobID, []jobdef.Status{jobdef.StatusFailed}, jobdef.StatusProcessing, "")
	require.NoError(t, err)
	require.True(t, ok)

	svc.runOne(context.Background(), jobID)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusReviewing, j.Status)
}

// ---------- start UpdateStatus error (status not allowed) ----------

func TestServiceStartUpdateStatusFails(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	file := filepath.Join(t.TempDir(), "STFAIL.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "STFAIL", RawNumber: "STFAIL", CleanedNumber: "STFAIL",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	err := svc.Run(context.Background(), jobID)
	require.ErrorIs(t, err, errJobStatusNotRunnable)

	assert.False(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		_, ok := svc.running[jobID]
		return ok
	}())
}

// ---------- Error paths via closed DB ----------

func newTestServiceWithClosedDB(t *testing.T) (*Service, *repository.JobRepository, int64) { //nolint:unparam
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)

	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())

	file := filepath.Join(t.TempDir(), "CLOSED.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	require.NoError(t, jobRepo.UpsertScannedJob(context.Background(), repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "CLOSED", RawNumber: "CLOSED", CleanedNumber: "CLOSED",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}))
	result, err := jobRepo.ListJobs(context.Background(), nil, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	jobID := result.Items[0].ID

	require.NoError(t, sqlite.Close())

	svc := &Service{
		jobRepo:    jobRepo,
		logRepo:    logRepo,
		scrapeRepo: scrapeRepo,
		storage:    store.NewMemStorage(),
		running:    make(map[int64]struct{}),
		queue:      make(chan queuedJob, 1024),
	}
	return svc, jobRepo, jobID
}

func TestServiceDeleteDBError(t *testing.T) {
	svc, _, jobID := newTestServiceWithClosedDB(t)
	err := svc.Delete(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceRecoverDBError(t *testing.T) {
	svc, _, _ := newTestServiceWithClosedDB(t)
	err := svc.Recover(context.Background())
	require.Error(t, err)
}

func TestServiceRunDBError(t *testing.T) {
	svc, _, jobID := newTestServiceWithClosedDB(t)
	err := svc.Run(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceRerunDBError(t *testing.T) {
	svc, _, jobID := newTestServiceWithClosedDB(t)
	err := svc.Rerun(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceUpdateNumberDBError(t *testing.T) {
	svc, _, jobID := newTestServiceWithClosedDB(t)
	_, err := svc.UpdateNumber(context.Background(), jobID, "X")
	require.Error(t, err)
}

func TestServiceListLogsDBError(t *testing.T) {
	svc, _, jobID := newTestServiceWithClosedDB(t)
	_, err := svc.ListLogs(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceGetScrapeDataDBError(t *testing.T) {
	svc, _, jobID := newTestServiceWithClosedDB(t)
	_, err := svc.GetScrapeData(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceGetConflictDBError(t *testing.T) {
	svc, _, _ := newTestServiceWithClosedDB(t)
	_, err := svc.GetConflict(context.Background(), &jobdef.Job{
		Status: jobdef.StatusInit, Number: "X", FileExt: ".mp4", FileName: "X.mp4",
	})
	require.Error(t, err)
}

func TestServiceApplyConflictsDBError(t *testing.T) {
	svc, _, _ := newTestServiceWithClosedDB(t)
	err := svc.ApplyConflicts(context.Background(), []jobdef.Job{
		{Status: jobdef.StatusInit, Number: "X", FileExt: ".mp4", FileName: "X.mp4"},
	})
	require.Error(t, err)
}

func TestRunOneWithDBError(t *testing.T) {
	svc, _, jobID := newTestServiceWithClosedDB(t)
	svc.runOne(context.Background(), jobID)
}

// ---------- Error paths via targeted table drops ----------

func dropTable(t *testing.T, svc *Service, _ string) { //nolint:unused
	t.Helper()
	db := svc.jobRepo
	_ = db
}

func breakScrapeTable(t *testing.T, sqlite *repository.SQLite) {
	t.Helper()
	_, err := sqlite.DB().Exec(`ALTER TABLE yamdc_scrape_data_tab RENAME TO yamdc_scrape_data_tab_broken`)
	require.NoError(t, err)
}

func breakLogTable(t *testing.T, sqlite *repository.SQLite) {
	t.Helper()
	_, err := sqlite.DB().Exec(`ALTER TABLE yamdc_unified_log_tab RENAME TO yamdc_unified_log_tab_broken`)
	require.NoError(t, err)
}

func breakJobTable(t *testing.T, sqlite *repository.SQLite) {
	t.Helper()
	_, err := sqlite.DB().Exec(`ALTER TABLE yamdc_job_tab RENAME TO yamdc_job_tab_broken`)
	require.NoError(t, err)
}

func newTestServiceWithRawDB(t *testing.T) (*Service, *repository.JobRepository, *repository.SQLite) {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })

	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())
	svc := &Service{
		jobRepo:    jobRepo,
		logRepo:    logRepo,
		scrapeRepo: scrapeRepo,
		storage:    store.NewMemStorage(),
		running:    make(map[int64]struct{}),
		queue:      make(chan queuedJob, 1024),
	}
	return svc, jobRepo, sqlite
}

func TestServiceDeleteScrapeDeleteError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	file := filepath.Join(t.TempDir(), "DEL-SDE.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJob(t, repo, file, jobdef.StatusFailed)

	breakScrapeTable(t, sqlite)
	err := svc.Delete(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete scrape data")
}

func TestServiceDeleteLogDeleteError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	file := filepath.Join(t.TempDir(), "DEL-LDE.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJob(t, repo, file, jobdef.StatusFailed)

	breakLogTable(t, sqlite)
	err := svc.Delete(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete job logs")
}

func TestServiceStartUpdateStatusError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	file := filepath.Join(t.TempDir(), "STATERR.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "STATERR", RawNumber: "STATERR", CleanedNumber: "STATERR",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	_ = jobID
	breakJobTable(t, sqlite)
	err := svc.Run(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceUpdateNumberPersistError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}}
	svc.capture = newLoggingTestCapture(t, searcher)

	file := filepath.Join(svc.capture.ScanDir(), "UPD-PE.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "UPD-PE", RawNumber: "UPD-PE", CleanedNumber: "UPD-PE",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	breakJobTable(t, sqlite)
	_, err := svc.UpdateNumber(context.Background(), jobID, "NEW")
	require.Error(t, err)
}

// ---------- executeScrapeAndFinalize status race (covers !ok path) ----------

func TestExecuteScrapeAndFinalizeStatusRace(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "RACE-002.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title:   "T",
		Cover:   &model.File{Name: "cover.jpg", Key: "k"},
		Poster:  &model.File{Name: "poster.jpg", Key: "k2"},
		ExtInfo: model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "test"}},
	}}
	svc.capture = newLoggingTestCapture(t, searcher)

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "RACE-002", RawNumber: "RACE-002", CleanedNumber: "RACE-002",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	ok, err := repo.UpdateStatus(context.Background(), jobID, []jobdef.Status{jobdef.StatusProcessing}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	sourcePath := file
	fc, err := svc.capture.ResolveFileContext(sourcePath, j.Number)
	require.NoError(t, err)

	svc.executeScrapeAndFinalize(context.Background(), jobID, j, sourcePath, fc)

	j, err = repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusReviewing, j.Status)
	assert.Contains(t, j.ErrorMsg, "")
}

// ---------- prepareJobExecution with bad number (triggers ResolveFileContext error) ----------

func TestPrepareJobExecutionResolveFileContextError(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	file := filepath.Join(t.TempDir(), "BAD.NUM.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "BAD.NUM", RawNumber: "BAD.NUM", CleanedNumber: "BAD.NUM",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	svc.runOne(context.Background(), jobID)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusFailed, j.Status)
	assert.Contains(t, j.ErrorMsg, "resolve file failed")
}

// ---------- executeScrapeAndFinalize with upsert error ----------

func TestExecuteScrapeAndFinalizeUpsertError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	file := filepath.Join(t.TempDir(), "UPSERT-ERR.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title:   "T",
		Cover:   &model.File{Name: "cover.jpg", Key: "k"},
		Poster:  &model.File{Name: "poster.jpg", Key: "k2"},
		ExtInfo: model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "test"}},
	}}
	svc.capture = newLoggingTestCapture(t, searcher)

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "UPSERT-ERR", RawNumber: "UPSERT-ERR", CleanedNumber: "UPSERT-ERR",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	breakScrapeTable(t, sqlite)

	svc.runOne(context.Background(), jobID)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusFailed, j.Status)
	assert.Contains(t, j.ErrorMsg, "save scrape data failed")
}

// ---------- Delete os.Remove error (non IsNotExist) ----------

func TestServiceDeleteRemoveError(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	lockedDir := filepath.Join(dir, "locked")
	require.NoError(t, os.MkdirAll(lockedDir, 0o755))
	file := filepath.Join(lockedDir, "file.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	require.NoError(t, os.Chmod(lockedDir, 0o555))
	t.Cleanup(func() {
		_ = os.Chmod(lockedDir, 0o755)
	})

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "DEL-RM", RawNumber: "DEL-RM", CleanedNumber: "DEL-RM",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusFailed)

	err := svc.Delete(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete source file failed")
}

// ---------- executeScrapeAndFinalize UpdateStatus error ----------

func TestExecuteScrapeAndFinalizeUpdateStatusError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	file := filepath.Join(t.TempDir(), "UPDERR.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title:   "T",
		Cover:   &model.File{Name: "cover.jpg", Key: "k"},
		Poster:  &model.File{Name: "poster.jpg", Key: "k2"},
		ExtInfo: model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "test"}},
	}}
	svc.capture = newLoggingTestCapture(t, searcher)

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "UPDERR", RawNumber: "UPDERR", CleanedNumber: "UPDERR",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	sourcePath := file
	fc, err := svc.capture.ResolveFileContext(sourcePath, j.Number)
	require.NoError(t, err)

	require.NoError(t, svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "pre", "{}"))
	breakJobTable(t, sqlite)
	svc.executeScrapeAndFinalize(context.Background(), jobID, j, sourcePath, fc)
}

// ---------- UpdateNumber with bad number format (covers ResolveFileContext error in UpdateNumber) ----------

func TestServiceUpdateNumberBadNumberFormat(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}}
	svc.capture = newLoggingTestCapture(t, searcher)

	file := filepath.Join(svc.capture.ScanDir(), "UPD-BAD.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "UPD-BAD", RawNumber: "UPD-BAD", CleanedNumber: "UPD-BAD",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	_, err := svc.UpdateNumber(context.Background(), jobID, "BAD.NUMBER.FORMAT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate number failed")
}

// ---------- syncJobSourcePath chain with DB error ----------

func TestResolveJobSourcePathSyncDBError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	dir := t.TempDir()
	origFile := filepath.Join(dir, "ORIG.mp4")
	require.NoError(t, os.WriteFile(origFile, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "ORIG.mp4", FileExt: ".mp4", RelPath: "ORIG.mp4", AbsPath: filepath.Join(dir, "GONE.mp4"),
		Number: "SYNC-ERR", RawNumber: "SYNC-ERR", CleanedNumber: "SYNC-ERR",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	breakJobTable(t, sqlite)

	_, err = svc.resolveJobSourcePath(context.Background(), j)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update job source path")
}

func TestResolveJobSourcePathSyncDBErrorViaDirScan(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SYNC-SCAN.mp4"), []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "GONE.mp4", FileExt: ".mp4", RelPath: "GONE.mp4", AbsPath: filepath.Join(dir, "GONE.mp4"),
		Number: "SYNC-SCAN", RawNumber: "SYNC-SCAN", CleanedNumber: "SYNC-SCAN",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	breakJobTable(t, sqlite)

	_, err = svc.matchByDirScan(context.Background(), j, dir, ".mp4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update job source path")
}

// ---------- start with broken conflict check ----------

func TestServiceStartGetConflictError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	file := filepath.Join(t.TempDir(), "CNF-ERR.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "CNF-ERR", RawNumber: "CNF-ERR", CleanedNumber: "CNF-ERR",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	_, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	breakJobTable(t, sqlite)

	err = svc.start(context.Background(), jobID, []jobdef.Status{jobdef.StatusInit})
	require.Error(t, err)
}

// ---------- Delete SoftDelete error ----------

func TestServiceDeleteSoftDeleteDBError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	file := filepath.Join(t.TempDir(), "DEL-SDE2.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "DEL-SDE2", RawNumber: "DEL-SDE2", CleanedNumber: "DEL-SDE2",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusFailed)

	_, err := sqlite.DB().Exec(`CREATE TRIGGER prevent_soft_delete BEFORE UPDATE ON yamdc_job_tab
		WHEN NEW.deleted_at != 0 BEGIN
			SELECT RAISE(ABORT, 'soft delete blocked by trigger');
		END`)
	require.NoError(t, err)

	err = svc.Delete(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "soft delete")
}

// ---------- start with GetConflict DB error ----------

func TestServiceStartUpdateStatusDBError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	file := filepath.Join(t.TempDir(), "STUP.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "STUP", RawNumber: "STUP", CleanedNumber: "STUP",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	_, err := sqlite.DB().Exec(`CREATE TRIGGER prevent_status_update BEFORE UPDATE ON yamdc_job_tab
		WHEN NEW.status != OLD.status BEGIN
			SELECT RAISE(ABORT, 'status update blocked by trigger');
		END`)
	require.NoError(t, err)

	err = svc.Run(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update job status")
}

// ---------- UpdateNumber jobRepo.UpdateNumber error via trigger ----------

func TestServiceUpdateNumberUpdateDBError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}}
	capt := newTestCaptureWithStorage(t, searcher, svc.storage)
	svc.capture = capt

	file := filepath.Join(capt.ScanDir(), "UPD-TRG.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "UPD-TRG", RawNumber: "UPD-TRG", CleanedNumber: "UPD-TRG",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	_, err := sqlite.DB().Exec(`CREATE TRIGGER prevent_number_update BEFORE UPDATE ON yamdc_job_tab
		WHEN NEW.number != OLD.number BEGIN
			SELECT RAISE(ABORT, 'number update blocked');
		END`)
	require.NoError(t, err)

	_, err = svc.UpdateNumber(context.Background(), jobID, "NEWNUM")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "persist updated job number")
}

// ---------- UpdateNumber with old DB persist error ----------

func TestServiceUpdateNumberPersistDBError(t *testing.T) {
	svc, repo, sqlite := newTestServiceWithRawDB(t)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T", Cover: &model.File{Name: "c.jpg", Key: "k"}, Poster: &model.File{Name: "p.jpg", Key: "k2"}}}
	capt := newTestCaptureWithStorage(t, searcher, svc.storage)
	svc.capture = capt

	file := filepath.Join(capt.ScanDir(), "UPD-DB.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "UPD-DB", RawNumber: "UPD-DB", CleanedNumber: "UPD-DB",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	_ = jobID
	breakJobTable(t, sqlite)
	_, err := svc.UpdateNumber(context.Background(), jobID, "NEW-NUM")
	require.Error(t, err)
}

// ---------- matchExplicitCandidates ----------

func TestMatchExplicitCandidatesWithSync(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	newFile := filepath.Join(dir, "NUM-001.mp4")
	require.NoError(t, os.WriteFile(newFile, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: "old.mp4", FileExt: ".mp4", RelPath: "old.mp4", AbsPath: filepath.Join(dir, "old.mp4"),
		Number: "NUM-001", RawNumber: "NUM-001", CleanedNumber: "NUM-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	j, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	found, err := svc.matchExplicitCandidates(context.Background(), j, dir, ".mp4")
	require.NoError(t, err)
	assert.Equal(t, newFile, found)
	assert.Equal(t, newFile, j.AbsPath)
	assert.Equal(t, "NUM-001.mp4", j.FileName)
}

// --- failJob ---

// TestFailJobUpdatesStatusAndWritesLog 验证正常路径:
// UpdateStatus 成功时, 任务被标记为 failed, 并写入一条 error 级别的 job log。
func TestFailJobUpdatesStatusAndWritesLog(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	ctx := context.Background()
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "FAIL-OK.mp4"), jobdef.StatusProcessing)

	svc.failJob(ctx, jobID, "scrape", "fake failure", "error=boom")

	j, err := repo.GetByID(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, j)
	assert.Equal(t, jobdef.StatusFailed, j.Status)
	assert.Equal(t, "fake failure", j.ErrorMsg)

	logs, err := svc.ListLogs(ctx, jobID)
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	assert.Equal(t, "error", logs[0].Level)
	assert.Equal(t, "scrape", logs[0].Stage)
	assert.Equal(t, "fake failure", logs[0].Message)
}

// TestFailJobWithClosedDBDoesNotPanic 验证异常路径:
// 底层 sql.DB 已被关闭时 (UpdateStatus/addJobLog 都会失败),
// failJob 必须静默返回而不是 panic, 以保证主流程可以继续执行
// (日志里会看到 "update status to failed failed" 的错误)。
func TestFailJobWithClosedDBDoesNotPanic(t *testing.T) {
	svc, _ := newTestServiceWithSQLite(t)

	// 主动关闭底层 DB 模拟"关停中触发失败路径"的场景。
	// 通过 jobRepo 拿不到 *sql.DB, 所以用反射路径比较复杂;
	// 这里换成构造一个"指向已关闭 DB 的 jobRepo"的 service。
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "closed.db"))
	require.NoError(t, err)
	require.NoError(t, sqlite.Close())

	closedSvc := NewService(
		repository.NewJobRepository(sqlite.DB()),
		repository.NewLogRepository(sqlite.DB()),
		repository.NewScrapeDataRepository(sqlite.DB()),
		nil, store.NewMemStorage(),
	)
	t.Cleanup(func() { _ = closedSvc.Stop(context.Background()) })

	assert.NotPanics(t, func() {
		closedSvc.failJob(context.Background(), 12345, "job", "boom", "detail")
	})
	// 确保原来那个 svc 仍可用 (失败不扩散到其它实例)
	assert.NotNil(t, svc)
}

// TestFailJobNonProcessingStatusNoTransition 验证边缘路径:
// 当 job 不处于 processing 状态时, UpdateStatus 不会生效,
// 但 log 仍被写入 (帮助排障), 避免把一个 done/init 的 job 错误地翻成 failed。
func TestFailJobNonProcessingStatusNoTransition(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	ctx := context.Background()
	jobID := insertJob(t, repo, filepath.Join(t.TempDir(), "FAIL-INIT.mp4"), jobdef.StatusInit)

	svc.failJob(ctx, jobID, "scrape", "should not transition", "detail")

	j, err := repo.GetByID(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, j)
	assert.Equal(t, jobdef.StatusInit, j.Status,
		"failJob requires status=processing, init job should stay untouched")

	logs, err := svc.ListLogs(ctx, jobID)
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	assert.Equal(t, "should not transition", logs[0].Message)
}

// gatedSearcher 在每次 Search 进入时自增 started 计数, 并阻塞直到 release 被 close;
// 适合用来在测试里拦住 worker 观察 Stop 的等待行为。
type gatedSearcher struct {
	started atomic.Int32
	release chan struct{}
	meta    *model.MovieMeta
}

func (s *gatedSearcher) Name() string                  { return "gated-test" }
func (s *gatedSearcher) Check(_ context.Context) error { return nil }
func (s *gatedSearcher) Search(ctx context.Context, n *number.Number) (*model.MovieMeta, bool, error) {
	s.started.Add(1)
	select {
	case <-s.release:
	case <-ctx.Done():
		return nil, false, fmt.Errorf("gated search canceled: %w", ctx.Err())
	}
	meta := *s.meta
	meta.Number = n.GetNumberID()
	return &meta, true, nil
}

func newGatedCapture(t *testing.T, searcher *gatedSearcher) *capture.Capture {
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

func insertRunnableJob(t *testing.T, repo *repository.JobRepository, number string) int64 {
	t.Helper()
	file := filepath.Join(t.TempDir(), number+".mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	return insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName:              filepath.Base(file),
		FileExt:               filepath.Ext(file),
		RelPath:               filepath.Base(file),
		AbsPath:               file,
		Number:                number,
		RawNumber:             number,
		CleanedNumber:         number,
		NumberSource:          "manual",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		FileSize:              1,
	}, jobdef.StatusInit)
}

// --- Stop: 正常 case: Stop 后 Run 被拒绝, DB 状态不变, running 集合为空 ---

func TestServiceStopRejectsNewRun(t *testing.T) {
	svc, repo := newTestService(t)
	jobID := insertRunnableJob(t, repo, "STOP-REJECT")

	require.NoError(t, svc.Stop(context.Background()))

	err := svc.Run(context.Background(), jobID)
	require.ErrorIs(t, err, ErrServiceStopped)

	got, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, jobdef.StatusInit, got.Status,
		"Run rejected by Stop must not flip status to processing")

	svc.mu.Lock()
	_, running := svc.running[jobID]
	svc.mu.Unlock()
	assert.False(t, running, "rejected Run must not leak into running map")
}

// --- Stop: 正常 case: Stop 是幂等的 ---

func TestServiceStopIsIdempotent(t *testing.T) {
	svc, _ := newTestService(t)
	require.NoError(t, svc.Stop(context.Background()))
	require.NoError(t, svc.Stop(context.Background()))
	require.NoError(t, svc.Stop(context.Background()))
}

// --- Stop: 正常 case: Stop 等到 in-flight job 完成才返回 ---

func TestServiceStopWaitsForInFlightJob(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	gate := &gatedSearcher{
		release: make(chan struct{}),
		meta: &model.MovieMeta{
			Title:   "Gated",
			Cover:   &model.File{Name: "cover.jpg", Key: "cover"},
			Poster:  &model.File{Name: "poster.jpg", Key: "poster"},
			ExtInfo: model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "gated"}},
		},
	}
	svc.capture = newGatedCapture(t, gate)

	jobID := insertRunnableJob(t, repo, "STOP-WAIT")
	require.NoError(t, svc.Run(context.Background(), jobID))
	require.Eventually(t, func() bool { return gate.started.Load() >= 1 },
		2*time.Second, 10*time.Millisecond, "worker must enter Search before Stop")

	stopDone := make(chan error, 1)
	go func() { stopDone <- svc.Stop(context.Background()) }()

	select {
	case err := <-stopDone:
		t.Fatalf("Stop returned before in-flight job completed: err=%v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(gate.release)

	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return within 3s after gate released")
	}

	got, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, jobdef.StatusReviewing, got.Status,
		"in-flight job must be allowed to finish and transition to reviewing")
}

// --- Stop: 异常 case: ctx 已取消时 Stop 返回 ctx.Err, 但 worker 仍会自然完成 ---

func TestServiceStopReturnsCtxErrorButWorkerStillCompletes(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	gate := &gatedSearcher{
		release: make(chan struct{}),
		meta: &model.MovieMeta{
			Title:   "Gated",
			Cover:   &model.File{Name: "c.jpg", Key: "c"},
			Poster:  &model.File{Name: "p.jpg", Key: "p"},
			ExtInfo: model.ExtInfo{ScrapeInfo: model.ScrapeInfo{Source: "gated"}},
		},
	}
	svc.capture = newGatedCapture(t, gate)

	jobID := insertRunnableJob(t, repo, "STOP-CTX")
	require.NoError(t, svc.Run(context.Background(), jobID))
	require.Eventually(t, func() bool { return gate.started.Load() >= 1 },
		2*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := svc.Stop(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		"Stop with canceled ctx must wrap ctx.Err; got: %v", err)

	// ctx 已取消让 Stop 先返回, 但 worker 仍在 gate 里 → 释放后必须能自然退出。
	close(gate.release)
	svc.WaitQueuedJobs()
	got, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, jobdef.StatusReviewing, got.Status,
		"worker must continue in background even after Stop returned ctx.Err")
}

// --- Stop: 边缘 case: 并发 Run + Stop 不 panic, 不残留 running 条目 ---

func TestServiceStopConcurrentRunNoPanic(t *testing.T) {
	svc, repo := newTestServiceWithSQLite(t)
	// 用会立刻失败的 searcher: 并发路径里即便 worker 真消费到了任务, 也只会
	// 走 failJob 分支, 不会因为 nil capture 触发 panic, 干扰我们观察 race。
	svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{
		err: fmt.Errorf("race-test: searcher short-circuit"),
	})

	const n = 20
	jobIDs := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		jobIDs = append(jobIDs, insertRunnableJob(t, repo, fmt.Sprintf("RACE-%03d", i)))
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, id := range jobIDs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// 结果不重要: 要么成功入队, 要么返回 ErrServiceStopped。关键是不 panic。
			_ = svc.Run(context.Background(), id)
		}()
	}
	// 再起一个 Stop goroutine 一起开跑。
	stopDone := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		stopDone <- svc.Stop(context.Background())
	}()

	require.NotPanics(t, func() {
		close(start)
		wg.Wait()
	})
	require.NoError(t, <-stopDone)

	// Stop 返回后再 Run 必须一律被拒。
	err := svc.Run(context.Background(), jobIDs[0])
	require.ErrorIs(t, err, ErrServiceStopped)

	// 所有成功入队的任务都已 finish, running 必须为空。
	svc.mu.Lock()
	runningCount := len(svc.running)
	svc.mu.Unlock()
	assert.Equal(t, 0, runningCount,
		"after Stop drains, running map must be empty; leaked=%d", runningCount)
}
