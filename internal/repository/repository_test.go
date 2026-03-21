package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/jobdef"
)

func newTestSQLite(t *testing.T) *SQLite {
	t.Helper()
	db, err := NewSQLite(filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	return db
}

func TestJobRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	err := repo.UpsertScannedJob(ctx, UpsertJobInput{
		FileName: "ABC-123.mp4",
		FileExt:  ".mp4",
		RelPath:  "a/ABC-123.mp4",
		AbsPath:  "/scan/a/ABC-123.mp4",
		Number:   "ABC-123",
		FileSize: 12345,
	})
	require.NoError(t, err)

	result, err := repo.ListJobs(ctx, []jobdef.Status{jobdef.StatusInit}, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, "ABC-123.mp4", result.Items[0].FileName)
	require.Equal(t, jobdef.StatusInit, result.Items[0].Status)

	ok, err := repo.UpdateStatus(ctx, result.Items[0].ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusProcessing, "")
	require.NoError(t, err)
	require.True(t, ok)

	got, err := repo.GetByID(ctx, result.Items[0].ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, jobdef.StatusProcessing, got.Status)

	err = repo.MarkDone(ctx, result.Items[0].ID)
	require.NoError(t, err)

	got, err = repo.GetByID(ctx, result.Items[0].ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, jobdef.StatusDone, got.Status)

	err = repo.SoftDelete(ctx, result.Items[0].ID)
	require.NoError(t, err)

	got, err = repo.GetByID(ctx, result.Items[0].ID)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestJobRepositoryRecoverProcessingJobs(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	err := repo.UpsertScannedJob(ctx, UpsertJobInput{
		FileName: "DEF-456.mkv",
		FileExt:  ".mkv",
		RelPath:  "DEF-456.mkv",
		AbsPath:  "/scan/DEF-456.mkv",
		Number:   "DEF-456",
		FileSize: 7,
	})
	require.NoError(t, err)

	result, err := repo.ListJobs(ctx, nil, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)

	ok, err := repo.UpdateStatus(ctx, result.Items[0].ID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusProcessing, "")
	require.NoError(t, err)
	require.True(t, ok)

	err = repo.RecoverProcessingJobs(ctx)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, result.Items[0].ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, jobdef.StatusFailed, got.Status)
	require.Equal(t, "server restarted while processing", got.ErrorMsg)
}

func TestLogAndScrapeDataRepository(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	logRepo := NewLogRepository(sqlite.DB())
	scrapeRepo := NewScrapeDataRepository(sqlite.DB())

	require.NoError(t, logRepo.Add(ctx, 1, "info", "scan", "scan started", "detail"))
	logs, err := logRepo.ListByJobID(ctx, 1, 10)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "scan started", logs[0].Message)

	require.NoError(t, scrapeRepo.UpsertRawData(ctx, 1, "plugin-a", `{"title":"a"}`))
	item, err := scrapeRepo.GetByJobID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, "plugin-a", item.Source)

	require.NoError(t, scrapeRepo.SaveReviewData(ctx, 1, `{"title":"b"}`))
	require.NoError(t, scrapeRepo.SaveFinalData(ctx, 1, `{"title":"c"}`))

	item, err = scrapeRepo.GetByJobID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, `{"title":"b"}`, item.ReviewData)
	require.Equal(t, `{"title":"c"}`, item.FinalData)
	require.Equal(t, "imported", item.Status)

	require.NoError(t, logRepo.DeleteByJobID(ctx, 1))
	require.NoError(t, scrapeRepo.DeleteByJobID(ctx, 1))

	logs, err = logRepo.ListByJobID(ctx, 1, 10)
	require.NoError(t, err)
	require.Len(t, logs, 0)

	item, err = scrapeRepo.GetByJobID(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, item)
}

func TestJobRepositoryListJobsWithKeywordAndPaging(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	for _, name := range []string{"AAA-001.mp4", "BBB-002.mp4", "AAA-003.mp4"} {
		require.NoError(t, repo.UpsertScannedJob(ctx, UpsertJobInput{
			FileName: name,
			FileExt:  ".mp4",
			RelPath:  name,
			AbsPath:  "/scan/" + name,
			Number:   name[:7],
			FileSize: 1,
		}))
	}

	result, err := repo.ListJobs(ctx, []jobdef.Status{jobdef.StatusInit}, "AAA", 1, 1)
	require.NoError(t, err)
	require.Equal(t, 2, result.Total)
	require.Equal(t, 1, result.Page)
	require.Equal(t, 1, result.PageSize)
	require.Len(t, result.Items, 1)

	result, err = repo.ListJobs(ctx, []jobdef.Status{jobdef.StatusInit}, "AAA", 2, 1)
	require.NoError(t, err)
	require.Equal(t, 2, result.Total)
	require.Len(t, result.Items, 1)
}
