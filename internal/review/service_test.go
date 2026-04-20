package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/capture"
	imgutil "github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
)

// ---------- helpers ----------

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

type noopProcessor struct{}

func (p *noopProcessor) Name() string                                          { return "noop" }
func (p *noopProcessor) Process(_ context.Context, _ *model.FileContext) error { return nil }

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

func testLogger() *zap.Logger {
	return zap.NewNop()
}

func makeTestJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// reviewTestRig 把 review.Service 运行所需的协作对象打包, 方便在不同测试
// 里以最小的样板构造完整的 reviewing 工作流测试环境。
type reviewTestRig struct {
	svc     *Service
	jobSvc  *job.Service
	jobRepo *repository.JobRepository
	sqlite  *repository.SQLite
}

func newTestRig(t *testing.T) *reviewTestRig {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())

	jobSvc := job.NewService(jobRepo, logRepo, scrapeRepo, nil, store.NewMemStorage())
	svc := NewService(jobSvc, jobRepo, scrapeRepo, nil, store.NewMemStorage())

	// 关闭顺序: 先等 job worker 把队列里的任务吃完, 再关 sqlite。
	t.Cleanup(func() { require.NoError(t, sqlite.Close()) })
	t.Cleanup(func() { jobSvc.WaitQueuedJobs() })
	return &reviewTestRig{svc: svc, jobSvc: jobSvc, jobRepo: jobRepo, sqlite: sqlite}
}

// newRigWithSharedStorage 让 review.Service 与 capture 共享同一个 storage,
// 方便 Import 路径下裁剪/读写 cover/poster 的测试。
func newRigWithSharedStorage(t *testing.T, storage store.IStorage) *reviewTestRig {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())

	jobSvc := job.NewService(jobRepo, logRepo, scrapeRepo, nil, storage)
	svc := NewService(jobSvc, jobRepo, scrapeRepo, nil, storage)

	t.Cleanup(func() { require.NoError(t, sqlite.Close()) })
	t.Cleanup(func() { jobSvc.WaitQueuedJobs() })
	return &reviewTestRig{svc: svc, jobSvc: jobSvc, jobRepo: jobRepo, sqlite: sqlite}
}

// newRigRawDB 不做 cleanup close, 交由调用方手动 break/close, 模拟 DB 故障。
func newRigRawDB(t *testing.T) *reviewTestRig {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())

	jobSvc := job.NewService(jobRepo, logRepo, scrapeRepo, nil, store.NewMemStorage())
	svc := NewService(jobSvc, jobRepo, scrapeRepo, nil, store.NewMemStorage())

	t.Cleanup(func() { _ = sqlite.Close() })
	t.Cleanup(func() { jobSvc.WaitQueuedJobs() })
	return &reviewTestRig{svc: svc, jobSvc: jobSvc, jobRepo: jobRepo, sqlite: sqlite}
}

// newRigClosedDB 预先创建一条 job 后关闭 DB, 用于触发 DB 读写失败路径。
func newRigClosedDB(t *testing.T) (*reviewTestRig, int64) {
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
	items, err := jobRepo.ListJobs(context.Background(), nil, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, items.Items, 1)
	jobID := items.Items[0].ID

	jobSvc := job.NewService(jobRepo, logRepo, scrapeRepo, nil, store.NewMemStorage())
	svc := NewService(jobSvc, jobRepo, scrapeRepo, nil, store.NewMemStorage())

	require.NoError(t, sqlite.Close())
	return &reviewTestRig{svc: svc, jobSvc: jobSvc, jobRepo: jobRepo, sqlite: sqlite}, jobID
}

func insertJob(t *testing.T, repo *repository.JobRepository, absPath string, status jobdef.Status) int64 {
	t.Helper()
	return insertJobWithInput(t, repo, repository.UpsertJobInput{
		FileName: filepath.Base(absPath), FileExt: filepath.Ext(absPath),
		RelPath: filepath.Base(absPath), AbsPath: absPath,
		Number: "TEST-001", RawNumber: "TEST001RAW", CleanedNumber: "TEST-001",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high",
		FileSize: 1,
	}, status)
}

func insertJobWithInput(t *testing.T, repo *repository.JobRepository, in repository.UpsertJobInput, status jobdef.Status) int64 {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.UpsertScannedJob(ctx, in))
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

func setupReviewingJobWithScrapeData(
	t *testing.T, rig *reviewTestRig, meta *model.MovieMeta,
) int64 {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "REVIEW-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: filepath.Ext(file),
		RelPath: filepath.Base(file), AbsPath: file,
		Number: "REVIEW-001", RawNumber: "REVIEW-001", CleanedNumber: "REVIEW-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high",
		FileSize: 1,
	}, jobdef.StatusReviewing)

	raw, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))
	return jobID
}

func breakScrapeTable(t *testing.T, sqlite *repository.SQLite) {
	t.Helper()
	_, err := sqlite.DB().Exec(`ALTER TABLE yamdc_scrape_data_tab RENAME TO yamdc_scrape_data_tab_broken`)
	require.NoError(t, err)
}

func breakJobTable(t *testing.T, sqlite *repository.SQLite) {
	t.Helper()
	_, err := sqlite.DB().Exec(`ALTER TABLE yamdc_job_tab RENAME TO yamdc_job_tab_broken`)
	require.NoError(t, err)
}

// ---------- SetImportGuard ----------

func TestServiceSetImportGuard(t *testing.T) {
	rig := newTestRig(t)
	assert.Nil(t, rig.svc.importGuard)
	rig.svc.SetImportGuard(func(_ context.Context) error { return nil })
	assert.NotNil(t, rig.svc.importGuard)
}

// ---------- SaveReviewData ----------

func TestServiceSaveReviewDataRequiresReviewing(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "C.mp4"), jobdef.StatusInit)

	err := rig.svc.SaveReviewData(context.Background(), jobID, `{"title":"ok"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reviewing")
}

func TestServiceSaveReviewDataRejectsInvalidJSON(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "D.mp4"), jobdef.StatusReviewing)

	err := rig.svc.SaveReviewData(context.Background(), jobID, `{`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid review json")
}

func TestServiceSaveReviewDataSuccess(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "T", Number: "SRD-001"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	err := rig.svc.SaveReviewData(context.Background(), jobID, `{"title":"Updated","number":"SRD-001"}`)
	require.NoError(t, err)

	data, err := rig.svc.scrapeRepo.GetByJobID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Contains(t, data.ReviewData, "Updated")
}

func TestServiceSaveReviewDataJobNotFound(t *testing.T) {
	rig := newTestRig(t)
	err := rig.svc.SaveReviewData(context.Background(), 99999, `{"title":"ok"}`)
	// 锁死"job 不存在"时返回的 sentinel 能被跨包 errors.Is 识别 —— 对应
	// 3.2 review 独立后 job.ErrJobNotFound = repository.ErrJobNotFound 的统一。
	require.ErrorIs(t, err, job.ErrJobNotFound)
	require.ErrorIs(t, err, repository.ErrJobNotFound)
}

func TestServiceSaveReviewDataValidJSON(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "T", Number: "SRD-OK"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	validJSON := `{"title":"Updated Title","number":"SRD-OK","actors":["Alice"]}`
	require.NoError(t, rig.svc.SaveReviewData(context.Background(), jobID, validJSON))
}

func TestServiceSaveReviewDataDBError(t *testing.T) {
	rig, jobID := newRigClosedDB(t)
	err := rig.svc.SaveReviewData(context.Background(), jobID, `{"title":"ok"}`)
	require.Error(t, err)
}

func TestServiceSaveReviewDataScrapeError(t *testing.T) {
	rig := newRigRawDB(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "SRD-ERR.mp4"), jobdef.StatusReviewing)

	breakScrapeTable(t, rig.sqlite)
	err := rig.svc.SaveReviewData(context.Background(), jobID, `{"title":"ok"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save review data")
}

// TestServiceSaveReviewDataBlockedByClaim: SaveReviewData 与 Import 共享同一
// 把 Claim 锁, 并发时后到者立即拿到 job.ErrJobAlreadyRunning, 不会把 scrape_data
// 写入半成品。这条用例覆盖 3.2 review 独立化后的并发互斥契约。
func TestServiceSaveReviewDataBlockedByClaim(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "SRD-LOCK.mp4"), jobdef.StatusReviewing)

	require.True(t, rig.jobSvc.Claim(jobID))
	defer rig.jobSvc.Finish(jobID)

	err := rig.svc.SaveReviewData(context.Background(), jobID, `{"title":"ok"}`)
	require.ErrorIs(t, err, job.ErrJobAlreadyRunning)
}

// TestServiceCropPosterFromCoverBlockedByClaim: 同上, 针对 CropPosterFromCover。
func TestServiceCropPosterFromCoverBlockedByClaim(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "CROP-LOCK.mp4"), jobdef.StatusReviewing)

	require.True(t, rig.jobSvc.Claim(jobID))
	defer rig.jobSvc.Finish(jobID)

	_, err := rig.svc.CropPosterFromCover(context.Background(), jobID, 0, 0, 10, 10)
	require.ErrorIs(t, err, job.ErrJobAlreadyRunning)
}

// ---------- loadReviewingMeta ----------

func TestLoadReviewingMetaJobNotFound(t *testing.T) {
	rig := newTestRig(t)
	_, err := rig.svc.loadReviewingMeta(context.Background(), testLogger(), 99999)
	require.ErrorIs(t, err, job.ErrJobNotFound)
	require.ErrorIs(t, err, repository.ErrJobNotFound)
}

func TestLoadReviewingMetaNotReviewingStatus(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "A.mp4"), jobdef.StatusInit)
	_, err := rig.svc.loadReviewingMeta(context.Background(), testLogger(), jobID)
	require.ErrorIs(t, err, ErrJobNotReviewing)
}

func TestLoadReviewingMetaScrapeDataNotFound(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "A.mp4"), jobdef.StatusReviewing)
	_, err := rig.svc.loadReviewingMeta(context.Background(), testLogger(), jobID)
	require.ErrorIs(t, err, repository.ErrScrapeDataNotFound)
}

func TestLoadReviewingMetaUsesReviewDataWhenPresent(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "Original", Number: "LRM-001"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	require.NoError(t, rig.svc.scrapeRepo.SaveReviewData(context.Background(), jobID,
		`{"title":"Reviewed","number":"LRM-001"}`))

	result, err := rig.svc.loadReviewingMeta(context.Background(), testLogger(), jobID)
	require.NoError(t, err)
	assert.Equal(t, "Reviewed", result.Title)
}

func TestLoadReviewingMetaUsesRawDataWhenNoReview(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "RawOnly", Number: "LRM-002"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	result, err := rig.svc.loadReviewingMeta(context.Background(), testLogger(), jobID)
	require.NoError(t, err)
	assert.Equal(t, "RawOnly", result.Title)
}

func TestLoadReviewingMetaInvalidJSON(t *testing.T) {
	rig := newTestRig(t)
	file := filepath.Join(t.TempDir(), "BADJSON.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "BADJSON", RawNumber: "BADJSON", CleanedNumber: "BADJSON",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", `{bad json!!!`))

	_, err := rig.svc.loadReviewingMeta(context.Background(), testLogger(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse review meta failed")
}

// ---------- CropPosterFromCover ----------

func TestServiceCropPosterFromCover(t *testing.T) {
	rig := newTestRig(t)
	coverData := makeTestJPEG(200, 300)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), rig.svc.storage, coverData)
	require.NoError(t, err)

	meta := &model.MovieMeta{
		Title: "Crop Test", Number: "CROP-001",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey},
	}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	poster, err := rig.svc.CropPosterFromCover(context.Background(), jobID, 10, 10, 50, 80)
	require.NoError(t, err)
	require.NotNil(t, poster)
	assert.Equal(t, "./poster.jpg", poster.Name)
	assert.NotEmpty(t, poster.Key)

	posterData, err := store.GetDataFrom(context.Background(), rig.svc.storage, poster.Key)
	require.NoError(t, err)
	img, err := imgutil.LoadImage(posterData)
	require.NoError(t, err)
	assert.Equal(t, 50, img.Bounds().Dx())
	assert.Equal(t, 80, img.Bounds().Dy())
}

func TestServiceCropPosterFromCoverNoCover(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "NoCover", Number: "NC-001"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	_, err := rig.svc.CropPosterFromCover(context.Background(), jobID, 0, 0, 10, 10)
	require.ErrorIs(t, err, ErrCoverNotFound)
}

func TestServiceCropPosterFromCoverOutOfBounds(t *testing.T) {
	rig := newTestRig(t)
	coverData := makeTestJPEG(100, 100)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), rig.svc.storage, coverData)
	require.NoError(t, err)

	meta := &model.MovieMeta{
		Title: "OOB Test", Number: "OOB-001",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey},
	}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	_, err = rig.svc.CropPosterFromCover(context.Background(), jobID, 0, 0, 200, 200)
	require.ErrorIs(t, err, ErrCropRectOutOfBounds)
}

func TestServiceCropPosterFromCoverNotReviewing(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "X.mp4"), jobdef.StatusInit)
	_, err := rig.svc.CropPosterFromCover(context.Background(), jobID, 0, 0, 10, 10)
	require.ErrorIs(t, err, ErrJobNotReviewing)
}

func TestServiceCropPosterFromCoverDBError(t *testing.T) {
	rig, jobID := newRigClosedDB(t)
	_, err := rig.svc.CropPosterFromCover(context.Background(), jobID, 0, 0, 10, 10)
	require.Error(t, err)
}

func TestCropAndStorePosterLoadFailed(t *testing.T) {
	rig := newTestRig(t)
	_, err := rig.svc.cropAndStorePoster(context.Background(), "nonexistent-key", 0, 0, 10, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load cover failed")
}

func TestCropAndStorePosterDecodeFailed(t *testing.T) {
	rig := newTestRig(t)
	err := store.PutDataTo(context.Background(), rig.svc.storage, "bad-image", []byte("not-an-image"))
	require.NoError(t, err)
	_, err = rig.svc.cropAndStorePoster(context.Background(), "bad-image", 0, 0, 10, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode cover failed")
}

func TestCropPosterFromCoverSaveError(t *testing.T) {
	rig := newRigRawDB(t)
	coverData := makeTestJPEG(200, 300)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), rig.svc.storage, coverData)
	require.NoError(t, err)

	dir := t.TempDir()
	file := filepath.Join(dir, "CROP-SAVE.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "CROP-SAVE", RawNumber: "CROP-SAVE", CleanedNumber: "CROP-SAVE",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{Title: "T", Number: "CROP-SAVE", Cover: &model.File{Name: "cover.jpg", Key: coverKey}}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	// 换表结构, 让后续 SaveReviewData 写入报错。
	_, err = rig.sqlite.DB().Exec(`DROP TABLE yamdc_scrape_data_tab;
		CREATE TABLE yamdc_scrape_data_tab (
			id INTEGER PRIMARY KEY,
			job_id INTEGER NOT NULL UNIQUE
		)`)
	require.NoError(t, err)

	_, err = rig.svc.CropPosterFromCover(context.Background(), jobID, 10, 10, 50, 80)
	require.Error(t, err)
}

// ---------- Import ----------

func TestServiceImportRequiresReviewing(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "E.mp4"), jobdef.StatusInit)

	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reviewing")
}

func TestServiceImportJobNotFound(t *testing.T) {
	rig := newTestRig(t)
	err := rig.svc.Import(context.Background(), 99999)
	// 与 SaveReviewData 同语义: review.Service 对"job 不存在"要同时满足
	// errors.Is(job.ErrJobNotFound) 和 errors.Is(repository.ErrJobNotFound),
	// 避免上层基于 sentinel 分支时漏命中。
	require.ErrorIs(t, err, job.ErrJobNotFound)
	require.ErrorIs(t, err, repository.ErrJobNotFound)
}

func TestServiceImportAlreadyRunning(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "IMP.mp4"), jobdef.StatusReviewing)
	// 通过 coordinator 抢占 slot, 模拟并发 Import。
	require.True(t, rig.jobSvc.Claim(jobID))
	defer rig.jobSvc.Finish(jobID)

	err := rig.svc.Import(context.Background(), jobID)
	require.ErrorIs(t, err, job.ErrJobAlreadyRunning)
}

func TestServiceImportBlockedByGuard(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "T", Number: "IMP-G"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	guardErr := errors.New("guard blocked")
	rig.svc.SetImportGuard(func(_ context.Context) error { return guardErr })

	err := rig.svc.Import(context.Background(), jobID)
	require.ErrorIs(t, err, guardErr)
}

// TestServiceImportBlockedByConflict 对应 3.2.b 修复后的新语义:
// 只有同 conflict_key 的兄弟 job 处于 processing / reviewing 时才阻塞 Import。
func TestServiceImportBlockedByConflict(t *testing.T) {
	rig := newTestRig(t)
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a@IMP-CNF.mp4")
	file2 := filepath.Join(dir, "b@IMP-CNF.mp4")
	require.NoError(t, os.WriteFile(file1, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(file2, []byte("x"), 0o600))

	insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file1), FileExt: ".mp4", RelPath: filepath.Base(file1), AbsPath: file1,
		Number: "IMP-CNF", RawNumber: "a@IMP-CNF", CleanedNumber: "IMP-CNF",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{Title: "T", Number: "IMP-CNF"}
	raw, _ := json.Marshal(meta)

	jobID2 := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file2), FileExt: ".mp4", RelPath: filepath.Base(file2), AbsPath: file2,
		Number: "IMP-CNF", RawNumber: "b@IMP-CNF", CleanedNumber: "IMP-CNF",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID2, "test", string(raw)))

	err := rig.svc.Import(context.Background(), jobID2)
	require.Error(t, err)
	require.ErrorIs(t, err, job.ErrJobConflict)
}

// TestServiceImportBlockedByProcessingPeer 对应 3.2.b: 兄弟 job 处于 processing
// (正在抓取, 尚未产出快照但已占用 slot) 时, 应当阻塞同 conflict_key 的 Import。
// 这条用例是对 getBlockingConflict 的"processing 分支"覆盖, 配合
// TestServiceImportBlockedByConflict (reviewing 分支) 一起锁死放宽语义。
func TestServiceImportBlockedByProcessingPeer(t *testing.T) {
	rig := newTestRig(t)
	dir := t.TempDir()

	peer := filepath.Join(dir, "peer@IMP-PROC.mp4")
	target := filepath.Join(dir, "target@IMP-PROC.mp4")
	require.NoError(t, os.WriteFile(peer, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(target, []byte("y"), 0o600))

	insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(peer), FileExt: ".mp4", RelPath: filepath.Base(peer), AbsPath: peer,
		Number: "IMP-PROC", RawNumber: "peer@IMP-PROC", CleanedNumber: "IMP-PROC",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusProcessing)

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(target), FileExt: ".mp4", RelPath: filepath.Base(target), AbsPath: target,
		Number: "IMP-PROC", RawNumber: "target@IMP-PROC", CleanedNumber: "IMP-PROC",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{Title: "T", Number: "IMP-PROC"}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
	require.ErrorIs(t, err, job.ErrJobConflict)
}

// TestServiceImportNotBlockedByInitPeer 对应 3.2.b: A=reviewing, B=init 不阻塞。
func TestServiceImportNotBlockedByInitPeer(t *testing.T) {
	storage := store.NewMemStorage()
	rig := newRigWithSharedStorage(t, storage)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(200, 300))
	require.NoError(t, err)
	posterKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(100, 150))
	require.NoError(t, err)

	capt := newTestCaptureWithStorage(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}}, storage)
	rig.svc.capture = capt
	dir := capt.ScanDir()

	peer := filepath.Join(dir, "peer@IMP-NOBLK.mp4")
	target := filepath.Join(dir, "target@IMP-NOBLK.mp4")
	require.NoError(t, os.WriteFile(peer, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(target, []byte("y"), 0o600))

	insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(peer), FileExt: ".mp4", RelPath: filepath.Base(peer), AbsPath: peer,
		Number: "IMP-NOBLK", RawNumber: "peer@IMP-NOBLK", CleanedNumber: "IMP-NOBLK",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusInit)

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(target), FileExt: ".mp4", RelPath: filepath.Base(target), AbsPath: target,
		Number: "IMP-NOBLK", RawNumber: "target@IMP-NOBLK", CleanedNumber: "IMP-NOBLK",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{
		Title: "Target", Number: "IMP-NOBLK",
		Cover:  &model.File{Name: "cover.jpg", Key: coverKey},
		Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	require.NoError(t, rig.svc.Import(context.Background(), jobID))
	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.Equal(t, jobdef.StatusDone, j.Status)
}

// TestServiceImportNotBlockedByFailedPeer 对应 3.2.b 边缘 case: failed 不阻塞。
func TestServiceImportNotBlockedByFailedPeer(t *testing.T) {
	storage := store.NewMemStorage()
	rig := newRigWithSharedStorage(t, storage)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(200, 300))
	require.NoError(t, err)
	posterKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(100, 150))
	require.NoError(t, err)

	capt := newTestCaptureWithStorage(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}}, storage)
	rig.svc.capture = capt
	dir := capt.ScanDir()

	peer := filepath.Join(dir, "peer@IMP-FAIL.mp4")
	target := filepath.Join(dir, "target@IMP-FAIL.mp4")
	require.NoError(t, os.WriteFile(peer, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(target, []byte("y"), 0o600))

	insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(peer), FileExt: ".mp4", RelPath: filepath.Base(peer), AbsPath: peer,
		Number: "IMP-FAIL", RawNumber: "peer", CleanedNumber: "IMP-FAIL",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high",
	}, jobdef.StatusFailed)

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(target), FileExt: ".mp4", RelPath: filepath.Base(target), AbsPath: target,
		Number: "IMP-FAIL", RawNumber: "target", CleanedNumber: "IMP-FAIL",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high",
	}, jobdef.StatusReviewing)
	meta := &model.MovieMeta{
		Title: "T", Number: "IMP-FAIL",
		Cover:  &model.File{Name: "cover.jpg", Key: coverKey},
		Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	require.NoError(t, rig.svc.Import(context.Background(), jobID))
}

func TestServiceImportNoScrapeData(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "IMP-NSD.mp4"), jobdef.StatusReviewing)
	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceImportDBError(t *testing.T) {
	rig, jobID := newRigClosedDB(t)
	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceImportInvalidJSON(t *testing.T) {
	rig := newTestRig(t)
	file := filepath.Join(t.TempDir(), "IMP-BAD-JSON.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-BAD-JSON", RawNumber: "IMP-BAD-JSON", CleanedNumber: "IMP-BAD-JSON",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", `{invalid-json`))

	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse final meta failed")
}

func TestServiceImportSourceNotFound(t *testing.T) {
	rig := newTestRig(t)
	rig.svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: "GONE.mp4", FileExt: ".mp4", RelPath: "GONE.mp4", AbsPath: "/nonexistent/GONE.mp4",
		Number: "GONE", RawNumber: "GONE", CleanedNumber: "GONE",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{Title: "T", Number: "GONE"}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestServiceImportRejectsSnapshotNumberMismatch 对应 3.2.a 第二道兜底。
func TestServiceImportRejectsSnapshotNumberMismatch(t *testing.T) {
	rig := newTestRig(t)
	rig.svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	dir := rig.svc.capture.ScanDir()
	file := filepath.Join(dir, "IMP-MISMATCH.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-NEW", RawNumber: "IMP-MISMATCH", CleanedNumber: "IMP-NEW",
		NumberSource: "cleaner", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	snapshot := &model.MovieMeta{Title: "Old", Number: "IMP-OLD"}
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	err = rig.svc.Import(context.Background(), jobID)
	require.ErrorIs(t, err, ErrScrapeDataNumberMismatch)
}

// TestServiceImportAllowsSnapshotNumberWithDifferentSeparators 对应 3.2.a 边缘 case:
// 比对时会先经 number.GetCleanID 去除 `-` / `_` + EqualFold 忽略大小写, 所以
// "SSIS-001" vs "ssis_001" vs "SSIS001" 这种仅分隔符/大小写差异的 number 不应
// 被误判为 mismatch。
func TestServiceImportAllowsSnapshotNumberWithDifferentSeparators(t *testing.T) {
	cases := []struct {
		name          string
		jobNumber     string
		snapshotValue string
	}{
		{name: "hyphen_vs_compact", jobNumber: "SSIS-001", snapshotValue: "SSIS001"},
		{name: "underscore_vs_hyphen", jobNumber: "SSIS-001", snapshotValue: "ssis_001"},
		{name: "case_only", jobNumber: "SSIS-001", snapshotValue: "ssis-001"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newTestRig(t)
			jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
				FileName: "S.mp4", FileExt: ".mp4", RelPath: "S.mp4", AbsPath: "/tmp/S.mp4",
				Number: tc.jobNumber, RawNumber: tc.jobNumber, CleanedNumber: tc.jobNumber,
				NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high",
			}, jobdef.StatusReviewing)
			j, err := rig.jobRepo.GetByID(context.Background(), jobID)
			require.NoError(t, err)

			raw, _ := json.Marshal(&model.MovieMeta{Title: "t", Number: tc.snapshotValue})
			err = rig.svc.verifyScrapeSnapshotMatchesJob(testLogger(), j, &repository.ScrapeData{
				RawData: string(raw),
			})
			require.NoError(t, err, "snapshot=%q vs job=%q should not be blocked", tc.snapshotValue, tc.jobNumber)
		})
	}
}

// TestServiceImportAllowsJobNumberWithVariantAgainstBaseSnapshot 锁死
// P0 fix: scrape 端只拿到基础番号 "PXVR-406", 而用户在 /processing 页手动
// 把 job number 填成带 variant 的写法 (CD / 4K / VR / LEAK / UC / ...) 时,
// verifyScrapeSnapshotMatchesJob 不能再按"字面量 + CleanID" 比对误判 mismatch,
// 必须先用 number.Parse 剥掉 variant, 只比较 base ID。
func TestServiceImportAllowsJobNumberWithVariantAgainstBaseSnapshot(t *testing.T) {
	cases := []struct {
		name          string
		jobNumber     string
		snapshotValue string
	}{
		{name: "cd2_vs_base", jobNumber: "PXVR-406-CD2", snapshotValue: "PXVR-406"},
		{name: "cd1_vs_base", jobNumber: "PXVR-406-CD1", snapshotValue: "PXVR-406"},
		{name: "chinese_sub_vs_base", jobNumber: "SSIS-001-C", snapshotValue: "SSIS-001"},
		{name: "4k_vs_base", jobNumber: "SSIS-001-4K", snapshotValue: "SSIS-001"},
		{name: "vr_vs_base", jobNumber: "PXVR-406-VR", snapshotValue: "PXVR-406"},
		{name: "leak_vs_base", jobNumber: "SSIS-001-LEAK", snapshotValue: "SSIS-001"},
		{name: "both_have_variant", jobNumber: "PXVR-406-CD2", snapshotValue: "pxvr406-cd1"},
		{name: "variant_differ_but_same_base", jobNumber: "SSIS-001-C", snapshotValue: "SSIS-001-4K"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newTestRig(t)
			jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
				FileName: "V.mp4", FileExt: ".mp4", RelPath: "V.mp4", AbsPath: "/tmp/V.mp4",
				Number: tc.jobNumber, RawNumber: tc.jobNumber, CleanedNumber: tc.jobNumber,
				NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high",
			}, jobdef.StatusReviewing)
			j, err := rig.jobRepo.GetByID(context.Background(), jobID)
			require.NoError(t, err)

			raw, _ := json.Marshal(&model.MovieMeta{Title: "t", Number: tc.snapshotValue})
			err = rig.svc.verifyScrapeSnapshotMatchesJob(testLogger(), j, &repository.ScrapeData{
				RawData: string(raw),
			})
			require.NoError(t, err,
				"job=%q vs snapshot=%q with same base should NOT be blocked after P0 fix",
				tc.jobNumber, tc.snapshotValue)
		})
	}
}

// TestServiceImportStillRejectsDifferentBaseNumber 确认 P0 fix 仍然拦住
// "base 都不同"的真实错位, 不能因为"忽略 variant"把兜底作用完全削掉。
func TestServiceImportStillRejectsDifferentBaseNumber(t *testing.T) {
	cases := []struct {
		name          string
		jobNumber     string
		snapshotValue string
	}{
		{name: "different_prefix", jobNumber: "PXVR-406-CD1", snapshotValue: "SSIS-001"},
		{name: "different_serial", jobNumber: "PXVR-406", snapshotValue: "PXVR-407"},
		{name: "different_serial_with_variant", jobNumber: "PXVR-406-CD1", snapshotValue: "PXVR-407-CD1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newTestRig(t)
			jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
				FileName: "V.mp4", FileExt: ".mp4", RelPath: "V.mp4", AbsPath: "/tmp/V.mp4",
				Number: tc.jobNumber, RawNumber: tc.jobNumber, CleanedNumber: tc.jobNumber,
				NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high",
			}, jobdef.StatusReviewing)
			j, err := rig.jobRepo.GetByID(context.Background(), jobID)
			require.NoError(t, err)

			raw, _ := json.Marshal(&model.MovieMeta{Title: "t", Number: tc.snapshotValue})
			err = rig.svc.verifyScrapeSnapshotMatchesJob(testLogger(), j, &repository.ScrapeData{
				RawData: string(raw),
			})
			require.ErrorIs(t, err, ErrScrapeDataNumberMismatch,
				"job=%q vs snapshot=%q with different base MUST be blocked",
				tc.jobNumber, tc.snapshotValue)
		})
	}
}

// TestServiceImportFallbackForUnparseableNumber 覆盖 canonicalBaseForCompare
// 的 Parse-fail 兜底路径: 当 number 带 `.` 这类 number.Parse 主动拒绝的字符时,
// 不应直接放行 / panic, 而应回落到"raw string 走 CleanID"的旧比对行为。
func TestServiceImportFallbackForUnparseableNumber(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: "F.mp4", FileExt: ".mp4", RelPath: "F.mp4", AbsPath: "/tmp/F.mp4",
		Number: "BAD.NUM", RawNumber: "BAD.NUM", CleanedNumber: "BAD.NUM",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high",
	}, jobdef.StatusReviewing)
	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)

	// 同样的坏字面量在两边, 走兜底的 GetCleanID 仍应判等 (两边都是 "BAD.NUM",
	// CleanID 只剥 `-`/`_` 所以原样返回) -> 不拦。
	rawSame, _ := json.Marshal(&model.MovieMeta{Title: "t", Number: "BAD.NUM"})
	require.NoError(t, rig.svc.verifyScrapeSnapshotMatchesJob(testLogger(), j, &repository.ScrapeData{
		RawData: string(rawSame),
	}))

	// base 真的不同时, 兜底路径也要能拦下来。
	rawDiff, _ := json.Marshal(&model.MovieMeta{Title: "t", Number: "OTHER.NUM"})
	err = rig.svc.verifyScrapeSnapshotMatchesJob(testLogger(), j, &repository.ScrapeData{
		RawData: string(rawDiff),
	})
	require.ErrorIs(t, err, ErrScrapeDataNumberMismatch)
}

// TestServiceImportTolerantToEmptyOrInvalidSnapshotNumber 对应 3.2.a 边缘 case。
func TestServiceImportTolerantToEmptyOrInvalidSnapshotNumber(t *testing.T) {
	cases := []struct {
		name   string
		rawRaw string
	}{
		{name: "empty_raw", rawRaw: ""},
		{name: "invalid_json", rawRaw: "{not json"},
		{name: "empty_number", rawRaw: `{"title":"x"}`},
		{name: "only_whitespace_number", rawRaw: `{"title":"x","number":"   "}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newTestRig(t)
			logger := testLogger()

			jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
				FileName: "SNP.mp4", FileExt: ".mp4", RelPath: "SNP.mp4", AbsPath: "/tmp/SNP.mp4",
				Number: "SNP-001", RawNumber: "SNP-001", CleanedNumber: "SNP-001",
				NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high",
			}, jobdef.StatusReviewing)
			j, err := rig.jobRepo.GetByID(context.Background(), jobID)
			require.NoError(t, err)

			err = rig.svc.verifyScrapeSnapshotMatchesJob(logger, j, &repository.ScrapeData{
				RawData: tc.rawRaw,
			})
			require.NoError(t, err, "snapshot=%q should not block import", tc.rawRaw)
		})
	}

	t.Run("nil_inputs_noop", func(t *testing.T) {
		rig := newTestRig(t)
		require.NoError(t, rig.svc.verifyScrapeSnapshotMatchesJob(testLogger(), nil, nil))
	})
}

func TestServiceImportMetaVerifyFailed(t *testing.T) {
	storage := store.NewMemStorage()
	rig := newRigWithSharedStorage(t, storage)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}}
	capt := newTestCaptureWithStorage(t, searcher, storage)
	rig.svc.capture = capt

	dir := capt.ScanDir()
	file := filepath.Join(dir, "IMP-VERIFY.mp4")
	require.NoError(t, os.WriteFile(file, []byte("movie"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-VERIFY", RawNumber: "IMP-VERIFY", CleanedNumber: "IMP-VERIFY",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 5,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{Title: "T", Number: "IMP-VERIFY"}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "import meta")
}

// ---------- Import full success path ----------

func TestServiceImportSuccess(t *testing.T) {
	storage := store.NewMemStorage()
	rig := newRigWithSharedStorage(t, storage)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(200, 300))
	require.NoError(t, err)
	posterKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(100, 150))
	require.NoError(t, err)

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title: "Import Title",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}}
	capt := newTestCaptureWithStorage(t, searcher, storage)
	rig.svc.capture = capt

	dir := capt.ScanDir()
	file := filepath.Join(dir, "IMP-OK-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("movie"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-OK-001", RawNumber: "IMP-OK-001", CleanedNumber: "IMP-OK-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 5,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{
		Title: "Import Title", Number: "IMP-OK-001",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	require.NoError(t, rig.svc.Import(context.Background(), jobID))

	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.Equal(t, jobdef.StatusDone, j.Status)

	data, err := rig.svc.scrapeRepo.GetByJobID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, "imported", data.Status)
}

func TestServiceImportWithReviewData(t *testing.T) {
	storage := store.NewMemStorage()
	rig := newRigWithSharedStorage(t, storage)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(200, 300))
	require.NoError(t, err)
	posterKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(100, 150))
	require.NoError(t, err)

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title: "Import Title",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}}
	capt := newTestCaptureWithStorage(t, searcher, storage)
	rig.svc.capture = capt

	dir := capt.ScanDir()
	file := filepath.Join(dir, "IMP-RV-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("movie"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-RV-001", RawNumber: "IMP-RV-001", CleanedNumber: "IMP-RV-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 5,
	}, jobdef.StatusReviewing)

	rawMeta := &model.MovieMeta{
		Title: "Original", Number: "IMP-RV-001",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}
	raw, _ := json.Marshal(rawMeta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	reviewMeta := &model.MovieMeta{
		Title: "Reviewed", Number: "IMP-RV-001",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}
	reviewRaw, _ := json.Marshal(reviewMeta)
	require.NoError(t, rig.svc.scrapeRepo.SaveReviewData(context.Background(), jobID, string(reviewRaw)))

	require.NoError(t, rig.svc.Import(context.Background(), jobID))

	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.Equal(t, jobdef.StatusDone, j.Status)
}

func TestServiceImportWithGuardPassing(t *testing.T) {
	storage := store.NewMemStorage()
	rig := newRigWithSharedStorage(t, storage)
	coverKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(200, 300))
	require.NoError(t, err)
	posterKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(100, 150))
	require.NoError(t, err)

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title: "T",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}}
	capt := newTestCaptureWithStorage(t, searcher, storage)
	rig.svc.capture = capt
	rig.svc.SetImportGuard(func(_ context.Context) error { return nil })

	dir := capt.ScanDir()
	file := filepath.Join(dir, "IMP-GD-001.mp4")
	require.NoError(t, os.WriteFile(file, []byte("movie"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-GD-001", RawNumber: "IMP-GD-001", CleanedNumber: "IMP-GD-001",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 5,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{
		Title: "T", Number: "IMP-GD-001",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	require.NoError(t, rig.svc.Import(context.Background(), jobID))
	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	require.Equal(t, jobdef.StatusDone, j.Status)
}

// ---------- Import error paths via broken tables ----------

func TestServiceImportValidatePreconditionsConflictError(t *testing.T) {
	rig := newRigRawDB(t)
	rig.svc.capture = newLoggingTestCapture(t, &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}})

	file := filepath.Join(t.TempDir(), "IMP-VPC.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-VPC", RawNumber: "IMP-VPC", CleanedNumber: "IMP-VPC",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{Title: "T", Number: "IMP-VPC"}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	breakJobTable(t, rig.sqlite)
	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceImportResolveFileContextError(t *testing.T) {
	storage := store.NewMemStorage()
	rig := newRigWithSharedStorage(t, storage)
	searcher := &loggingTestSearcher{meta: &model.MovieMeta{Title: "T"}}
	capt := newTestCaptureWithStorage(t, searcher, storage)
	rig.svc.capture = capt

	dir := capt.ScanDir()
	file := filepath.Join(dir, "BAD.NUM.mp4")
	require.NoError(t, os.WriteFile(file, []byte("movie"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "BAD.NUM", RawNumber: "BAD.NUM", CleanedNumber: "BAD.NUM",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{Title: "T", Number: "BAD.NUM"}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve file context")
}

func TestServiceImportMarkDoneError(t *testing.T) {
	rig := newRigRawDB(t)
	storage := store.NewMemStorage()
	coverKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(200, 300))
	require.NoError(t, err)
	posterKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(100, 150))
	require.NoError(t, err)

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title: "T", Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}}
	capt := newTestCaptureWithStorage(t, searcher, storage)
	rig.svc.capture = capt
	rig.svc.storage = storage

	dir := capt.ScanDir()
	file := filepath.Join(dir, "IMP-MDE.mp4")
	require.NoError(t, os.WriteFile(file, []byte("movie"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-MDE", RawNumber: "IMP-MDE", CleanedNumber: "IMP-MDE",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 5,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{
		Title: "T", Number: "IMP-MDE",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	breakJobTable(t, rig.sqlite)
	err = rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
}

func TestServiceImportScrapeDataGetError(t *testing.T) {
	rig := newRigRawDB(t)

	file := filepath.Join(t.TempDir(), "IMP-SGE.mp4")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-SGE", RawNumber: "IMP-SGE", CleanedNumber: "IMP-SGE",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 1,
	}, jobdef.StatusReviewing)

	breakScrapeTable(t, rig.sqlite)
	err := rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get scrape data")
}

func TestServiceImportSaveFinalDataTriggerError(t *testing.T) {
	rig := newRigRawDB(t)
	storage := store.NewMemStorage()
	coverKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(200, 300))
	require.NoError(t, err)
	posterKey, err := store.AnonymousPutDataTo(context.Background(), storage, makeTestJPEG(100, 150))
	require.NoError(t, err)

	searcher := &loggingTestSearcher{meta: &model.MovieMeta{
		Title: "T", Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}}
	capt := newTestCaptureWithStorage(t, searcher, storage)
	rig.svc.capture = capt
	rig.svc.storage = storage

	dir := capt.ScanDir()
	file := filepath.Join(dir, "IMP-TRG.mp4")
	require.NoError(t, os.WriteFile(file, []byte("movie"), 0o600))

	jobID := insertJobWithInput(t, rig.jobRepo, repository.UpsertJobInput{
		FileName: filepath.Base(file), FileExt: ".mp4", RelPath: filepath.Base(file), AbsPath: file,
		Number: "IMP-TRG", RawNumber: "IMP-TRG", CleanedNumber: "IMP-TRG",
		NumberSource: "manual", NumberCleanStatus: "success", NumberCleanConfidence: "high", FileSize: 5,
	}, jobdef.StatusReviewing)

	meta := &model.MovieMeta{
		Title: "T", Number: "IMP-TRG",
		Cover: &model.File{Name: "cover.jpg", Key: coverKey}, Poster: &model.File{Name: "poster.jpg", Key: posterKey},
	}
	raw, _ := json.Marshal(meta)
	require.NoError(t, rig.svc.scrapeRepo.UpsertRawData(context.Background(), jobID, "test", string(raw)))

	_, err = rig.sqlite.DB().Exec(`CREATE TRIGGER prevent_final_data BEFORE UPDATE ON yamdc_scrape_data_tab
		WHEN NEW.final_data != '' BEGIN
			SELECT RAISE(ABORT, 'final data update blocked');
		END`)
	require.NoError(t, err)

	err = rig.svc.Import(context.Background(), jobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save final data")
}

// ---------- Reject ----------

// TestServiceRejectSuccess 覆盖 Reject 的主路径: reviewing -> failed, scrape
// 数据被清空, error_msg = 传入 reason。
func TestServiceRejectSuccess(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "T", Number: "RJ-001"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	require.NoError(t, rig.svc.Reject(context.Background(), jobID, "用户手动打回"))

	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusFailed, j.Status)
	assert.Equal(t, "用户手动打回", j.ErrorMsg)

	_, err = rig.svc.scrapeRepo.GetByJobID(context.Background(), jobID)
	// scrape_data 被 DeleteByJobID 删掉, 下一次读应该拿到 NotFound。
	require.ErrorIs(t, err, repository.ErrScrapeDataNotFound)
}

// TestServiceRejectUsesDefaultReasonWhenEmpty 锁定"空 reason -> 默认文案"。
func TestServiceRejectUsesDefaultReasonWhenEmpty(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "T", Number: "RJ-DEF"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	require.NoError(t, rig.svc.Reject(context.Background(), jobID, "   "))
	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusFailed, j.Status)
	assert.Equal(t, "rejected by reviewer", j.ErrorMsg)
}

// TestServiceRejectTruncatesOverlongReason 锁定"超长 reason 被截断到 200 + ..."。
// UI 层目前不做输入上限, 后端要给个硬上限防止 SQL 层膨胀 / 日志膨胀。
func TestServiceRejectTruncatesOverlongReason(t *testing.T) {
	rig := newTestRig(t)
	meta := &model.MovieMeta{Title: "T", Number: "RJ-LONG"}
	jobID := setupReviewingJobWithScrapeData(t, rig, meta)

	long := strings.Repeat("x", 500)
	require.NoError(t, rig.svc.Reject(context.Background(), jobID, long))
	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, 203, len(j.ErrorMsg)) // 200 + "..."
	assert.True(t, strings.HasSuffix(j.ErrorMsg, "..."))
}

// TestServiceRejectJobNotFound: 上游用 errors.Is 检测"job 不存在"要能命中,
// 保持和 Import/SaveReviewData 一致。
func TestServiceRejectJobNotFound(t *testing.T) {
	rig := newTestRig(t)
	err := rig.svc.Reject(context.Background(), 99999, "x")
	require.ErrorIs(t, err, job.ErrJobNotFound)
	require.ErrorIs(t, err, repository.ErrJobNotFound)
}

// TestServiceRejectRequiresReviewing: 非 reviewing 状态下拒绝打回。init / failed /
// done / processing 四个状态都应该返回 ErrJobNotReviewing。
func TestServiceRejectRequiresReviewing(t *testing.T) {
	cases := []jobdef.Status{
		jobdef.StatusInit,
		jobdef.StatusFailed,
		jobdef.StatusDone,
		jobdef.StatusProcessing,
	}
	for _, status := range cases {
		t.Run(string(status), func(t *testing.T) {
			rig := newTestRig(t)
			jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), string(status)+".mp4"), status)
			err := rig.svc.Reject(context.Background(), jobID, "x")
			require.ErrorIs(t, err, ErrJobNotReviewing)
		})
	}
}

// TestServiceRejectAlreadyRunning: Claim 失败时立即返回 ErrJobAlreadyRunning,
// 不会读库。
func TestServiceRejectAlreadyRunning(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "RJ-LOCK.mp4"), jobdef.StatusReviewing)
	require.True(t, rig.jobSvc.Claim(jobID))
	defer rig.jobSvc.Finish(jobID)

	err := rig.svc.Reject(context.Background(), jobID, "x")
	require.ErrorIs(t, err, job.ErrJobAlreadyRunning)
}

// TestServiceRejectWithoutScrapeData: 即使从来没写过 scrape_data, Reject 仍应
// 成功 (DeleteByJobID 对空表幂等)。
func TestServiceRejectWithoutScrapeData(t *testing.T) {
	rig := newTestRig(t)
	jobID := insertJob(t, rig.jobRepo, filepath.Join(t.TempDir(), "RJ-NOSD.mp4"), jobdef.StatusReviewing)

	require.NoError(t, rig.svc.Reject(context.Background(), jobID, "nothing scraped"))
	j, err := rig.jobRepo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusFailed, j.Status)
}

// TestServiceRejectDBError: DB 关闭时 Reject 报错, 不 panic。
func TestServiceRejectDBError(t *testing.T) {
	rig, jobID := newRigClosedDB(t)
	// newRigClosedDB 造出的 job 是 init 态, 先切到 reviewing 再关库;
	// 但此处 sqlite 已经关了, 所以我们直接期望 load job 报错。
	err := rig.svc.Reject(context.Background(), jobID, "x")
	require.Error(t, err)
}
