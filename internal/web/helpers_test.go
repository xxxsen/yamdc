package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type testHandlerFn func(ctx context.Context, fc *model.FileContext) error

func (fn testHandlerFn) Handle(ctx context.Context, fc *model.FileContext) error {
	return fn(ctx, fc)
}

func phandlerDebugRuntime() appdeps.Runtime {
	return appdeps.Runtime{
		Storage: store.NewMemStorage(),
	}
}

type stubCleaner struct {
	explainResult *movieidcleaner.ExplainResult
	explainErr    error
}

func (*stubCleaner) Clean(_ string) (*movieidcleaner.Result, error) { return nil, nil } //nolint:nilnil
func (s *stubCleaner) Explain(_ string) (*movieidcleaner.ExplainResult, error) {
	return s.explainResult, s.explainErr
}

func newGinContext(method, target string, body io.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequestWithContext(context.Background(), method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx.Request = req
	return ctx, rec
}

func newGinContextWithParams(method, target string, body io.Reader, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	c, rec := newGinContext(method, target, body)
	c.Params = params
	return c, rec
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) responseBody {
	t.Helper()
	var out responseBody
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	return out
}

func buildMultipartImage(t *testing.T, fieldName, fileName string, data []byte) (*bytes.Buffer, string) { //nolint:unparam
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func pngBytes() []byte {
	return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
}

func jpegBytes() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xdb}
}

func setupTestDB(t *testing.T) (*repository.SQLite, *repository.JobRepository, *repository.LogRepository, *repository.ScrapeDataRepository) { //nolint:unparam
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlite, err := repository.NewSQLite(context.Background(), dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })
	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())
	return sqlite, jobRepo, logRepo, scrapeRepo
}

// newTestJobService 创建 job.Service 并注册清理逻辑, 等待 worker goroutine
// 在 sqlite 关闭/tempdir 清理之前写完所有异步日志, 避免与 t.TempDir()
// 的 RemoveAll 产生 "directory not empty" 竞态。
func newTestJobService(
	t *testing.T,
	jobRepo *repository.JobRepository,
	logRepo *repository.LogRepository,
	scrapeRepo *repository.ScrapeDataRepository,
	storage store.IStorage,
) *job.Service {
	t.Helper()
	svc := job.NewService(jobRepo, logRepo, scrapeRepo, nil, storage)
	t.Cleanup(func() {
		svc.WaitQueuedJobs()
	})
	return svc
}

func createTestJob(t *testing.T, jobRepo *repository.JobRepository, num string) *jobdef.Job {
	t.Helper()
	err := jobRepo.UpsertScannedJob(context.Background(), repository.UpsertJobInput{
		FileName:      num + ".mp4",
		FileExt:       ".mp4",
		RelPath:       num + ".mp4",
		AbsPath:       filepath.Join(t.TempDir(), num+".mp4"),
		Number:        num,
		RawNumber:     num,
		CleanedNumber: num,
		NumberSource:  "raw",
		FileSize:      1,
	})
	require.NoError(t, err)
	items, err := jobRepo.ListJobs(context.Background(), []jobdef.Status{jobdef.StatusInit}, "", 1, 50)
	require.NoError(t, err)
	for _, item := range items.Items {
		if item.Number == num {
			return &item
		}
	}
	t.Fatalf("job not found for number %s", num)
	return nil
}

func setupMediaLibDB(t *testing.T) *medialib.Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "media.db")
	sqlite, err := repository.NewSQLite(context.Background(), dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })
	libraryDir := t.TempDir()
	svc := medialib.NewService(sqlite.DB(), libraryDir, "")
	// 按 LIFO, 先等待后台 sync/move goroutine 返回再关闭 sqlite/tempdir,
	// 避免 TriggerFullSync / TriggerMove 启动的异步 DB 写入与清理竞争。
	t.Cleanup(func() { svc.WaitBackground() })
	return svc
}

func createValidJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}))
	return buf.Bytes()
}

type failingStore struct{}

func (f *failingStore) GetData(_ context.Context, _ string) ([]byte, error) {
	return nil, fmt.Errorf("store get error")
}

func (f *failingStore) PutData(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return fmt.Errorf("store put error")
}

func (f *failingStore) IsDataExist(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) { return 0, fmt.Errorf("read error") }

func setupClosedMediaLibDB(t *testing.T) *medialib.Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "closed.db")
	sqlite, err := repository.NewSQLite(context.Background(), dbPath)
	require.NoError(t, err)
	svc := medialib.NewService(sqlite.DB(), t.TempDir(), t.TempDir())
	require.NoError(t, sqlite.Close())
	return svc
}

func setupClosedJobDB(t *testing.T) (*repository.JobRepository, *repository.LogRepository, *repository.ScrapeDataRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "closed_job.db")
	sqlite, err := repository.NewSQLite(context.Background(), dbPath)
	require.NoError(t, err)
	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())
	require.NoError(t, sqlite.Close())
	return jobRepo, logRepo, scrapeRepo
}
