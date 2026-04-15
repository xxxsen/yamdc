package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/jobdef"
)

func newTestSQLite(t *testing.T) *SQLite {
	t.Helper()
	db, err := NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
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
	require.ErrorIs(t, err, ErrJobNotFound)
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
	require.ErrorIs(t, err, ErrScrapeDataNotFound)
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

func TestJobRepositoryUpsertScannedJobDoesNotRefreshUpdatedAtWithoutChanges(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	input := UpsertJobInput{
		FileName: "AAA-001.mp4",
		FileExt:  ".mp4",
		RelPath:  "AAA-001.mp4",
		AbsPath:  "/scan/AAA-001.mp4",
		Number:   "AAA-001",
		FileSize: 1,
	}
	require.NoError(t, repo.UpsertScannedJob(ctx, input))

	result, err := repo.ListJobs(ctx, []jobdef.Status{jobdef.StatusInit}, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	firstUpdatedAt := result.Items[0].UpdatedAt

	require.NoError(t, repo.UpsertScannedJob(ctx, input))

	result, err = repo.ListJobs(ctx, []jobdef.Status{jobdef.StatusInit}, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, firstUpdatedAt, result.Items[0].UpdatedAt)
}

func TestJobRepositoryUpsertScannedJobPreservesManualNumber(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	input := UpsertJobInput{
		FileName:              "AAA-001.mp4",
		FileExt:               ".mp4",
		RelPath:               "AAA-001.mp4",
		AbsPath:               "/scan/AAA-001.mp4",
		Number:                "AAA-001",
		RawNumber:             "AAA001-raw",
		CleanedNumber:         "AAA-001",
		NumberSource:          "cleaner",
		NumberCleanStatus:     "success",
		NumberCleanConfidence: "high",
		NumberCleanWarnings:   "",
		FileSize:              1,
	}
	require.NoError(t, repo.UpsertScannedJob(ctx, input))

	result, err := repo.ListJobs(ctx, []jobdef.Status{jobdef.StatusInit}, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	jobID := result.Items[0].ID

	require.NoError(t, repo.UpdateNumber(ctx, jobID, "MANUAL-999", "manual", "success", "high", ""))

	input.CleanedNumber = "AAA-002"
	input.Number = "AAA-002"
	input.RawNumber = "AAA002-raw"
	require.NoError(t, repo.UpsertScannedJob(ctx, input))

	got, err := repo.GetByID(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "MANUAL-999", got.Number)
	require.Equal(t, "manual", got.NumberSource)
	require.Equal(t, "AAA-002", got.CleanedNumber)
	require.Equal(t, "AAA002-raw", got.RawNumber)
	require.Equal(t, "MANUAL-999.mp4", got.ConflictKey)
}

func TestJobRepositoryUpsertScannedJobReactivatesDoneJob(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	input := UpsertJobInput{
		FileName: "AAA-001.mp4",
		FileExt:  ".mp4",
		RelPath:  "AAA-001.mp4",
		AbsPath:  "/scan/AAA-001.mp4",
		Number:   "AAA-001",
		FileSize: 1,
	}
	require.NoError(t, repo.UpsertScannedJob(ctx, input))

	result, err := repo.ListJobs(ctx, nil, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	jobID := result.Items[0].ID

	require.NoError(t, repo.MarkDone(ctx, jobID))

	require.NoError(t, repo.UpsertScannedJob(ctx, input))

	got, err := repo.GetByID(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, jobdef.StatusInit, got.Status)
	require.Equal(t, "", got.ErrorMsg)
}

func TestJobRepositoryStoresAndUpdatesConflictKey(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	require.NoError(t, repo.UpsertScannedJob(ctx, UpsertJobInput{
		FileName: "ABC-123.mp4",
		FileExt:  ".mp4",
		RelPath:  "ABC-123.mp4",
		AbsPath:  "/scan/ABC-123.mp4",
		Number:   "ABC-123",
		FileSize: 1,
	}))

	result, err := repo.ListJobs(ctx, nil, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	jobID := result.Items[0].ID

	got, err := repo.GetByID(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "ABC-123.mp4", got.ConflictKey)

	require.NoError(t, repo.UpdateNumber(ctx, jobID, "XYZ-999", "manual", "success", "high", ""))
	got, err = repo.GetByID(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "XYZ-999.mp4", got.ConflictKey)
}

func TestJobRepositoryUpdateSourcePathRefreshesConflictKey(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	require.NoError(t, repo.UpsertScannedJob(ctx, UpsertJobInput{
		FileName: "ABC-123.mp4",
		FileExt:  ".mp4",
		RelPath:  "ABC-123.mp4",
		AbsPath:  "/scan/ABC-123.mp4",
		Number:   "ABC-123",
		FileSize: 1,
	}))

	result, err := repo.ListJobs(ctx, nil, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	jobID := result.Items[0].ID

	require.NoError(t, repo.UpdateSourcePath(ctx, jobID, "ABC-123.mkv", ".mkv", "ABC-123.mkv", "/scan/ABC-123.mkv"))

	got, err := repo.GetByID(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, ".mkv", got.FileExt)
	require.Equal(t, "ABC-123.mkv", got.ConflictKey)
}

func TestJobRepositoryListActiveJobsByConflictKeysFiltersDoneAndDeleted(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	for _, relPath := range []string{"a/ABC-123.mp4", "b/ABC-123.mp4", "c/ABC-123.mp4", "d/ABC-123.mp4"} {
		require.NoError(t, repo.UpsertScannedJob(ctx, UpsertJobInput{
			FileName: filepath.Base(relPath),
			FileExt:  ".mp4",
			RelPath:  relPath,
			AbsPath:  "/scan/" + relPath,
			Number:   "ABC-123",
			FileSize: 1,
		}))
	}

	result, err := repo.ListJobs(ctx, nil, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 4)

	require.NoError(t, repo.MarkDone(ctx, result.Items[1].ID))
	require.NoError(t, repo.SoftDelete(ctx, result.Items[2].ID))

	items, err := repo.ListActiveJobsByConflictKeys(ctx, []string{"ABC-123.mp4"})
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		require.Equal(t, "ABC-123.mp4", item.ConflictKey)
		require.NotEqual(t, result.Items[1].ID, item.ID)
		require.NotEqual(t, result.Items[2].ID, item.ID)
	}
}

// ---------- helpers ----------

func newClosedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "closed.db")
	s, err := NewSQLite(context.Background(), path)
	require.NoError(t, err)
	db := s.DB()
	require.NoError(t, db.Close())
	return db
}

func newTestSQLiteWithRawDB(t *testing.T) (*SQLite, *sql.DB) { //nolint:unparam
	t.Helper()
	s := newTestSQLite(t)
	return s, s.DB()
}

func insertTestJob(t *testing.T, repo *JobRepository, relPath, number string) int64 {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.UpsertScannedJob(ctx, UpsertJobInput{
		FileName: filepath.Base(relPath),
		FileExt:  filepath.Ext(relPath),
		RelPath:  relPath,
		AbsPath:  "/scan/" + relPath,
		Number:   number,
		FileSize: 1,
	}))
	result, err := repo.ListJobs(ctx, nil, "", 1, 100)
	require.NoError(t, err)
	for _, item := range result.Items {
		if item.RelPath == relPath {
			return item.ID
		}
	}
	t.Fatalf("job not found for relPath=%s", relPath)
	return 0
}

// ==================== SQLite tests ====================

func TestNewSQLiteMkdirAllError(t *testing.T) {
	_, err := NewSQLite(context.Background(), "/dev/null/impossible/test.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create db dir")
}

func TestNewSQLiteCorruptDBInitError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "corrupt.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("this is not a valid sqlite database file"), 0o600))
	_, err := NewSQLite(context.Background(), dbPath)
	require.Error(t, err)
}

func TestConfigureSQLiteNilDB(_ *testing.T) {
	configureSQLite(context.Background(), nil)
}

func TestSQLiteCloseNilDB(t *testing.T) {
	s := &SQLite{db: nil}
	require.NoError(t, s.Close())
}

// ==================== ListJobs edge cases ====================

func TestListJobsPageZeroDefaultsToOne(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())
	insertTestJob(t, repo, "A.mp4", "A")

	result, err := repo.ListJobs(ctx, nil, "", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Page)
	assert.Len(t, result.Items, 1)
}

func TestListJobsPageSizeCappedAt200(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())
	insertTestJob(t, repo, "A.mp4", "A")

	result, err := repo.ListJobs(ctx, nil, "", 1, 300)
	require.NoError(t, err)
	assert.Equal(t, 200, result.PageSize)
}

func TestListJobsAllMode(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())
	for _, name := range []string{"f0.mp4", "f1.mp4", "f2.mp4"} {
		insertTestJob(t, repo, name, name[:2])
	}

	result, err := repo.ListJobs(ctx, nil, "", 1, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 3, result.PageSize)
	assert.Len(t, result.Items, 3)
}

func TestListJobsMultipleStatuses(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())
	id1 := insertTestJob(t, repo, "A.mp4", "A")
	id2 := insertTestJob(t, repo, "B.mp4", "B")

	ok, err := repo.UpdateStatus(ctx, id1, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusProcessing, "")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.UpdateStatus(ctx, id2, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusFailed, "oops")
	require.NoError(t, err)
	require.True(t, ok)

	result, err := repo.ListJobs(ctx, []jobdef.Status{jobdef.StatusProcessing, jobdef.StatusFailed}, "", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
}

// ==================== UpdateStatus edge cases ====================

func TestUpdateStatusMultipleFromStatuses(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())
	id := insertTestJob(t, repo, "A.mp4", "A")

	ok, err := repo.UpdateStatus(ctx, id, []jobdef.Status{jobdef.StatusInit, jobdef.StatusFailed}, jobdef.StatusProcessing, "")
	require.NoError(t, err)
	require.True(t, ok)

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, jobdef.StatusProcessing, got.Status)
}

// ==================== ListActiveJobsByConflictKeys edge cases ====================

func TestListActiveJobsByConflictKeysEdgeCases(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	repo := NewJobRepository(sqlite.DB())

	t.Run("empty_key", func(t *testing.T) {
		items, err := repo.ListActiveJobsByConflictKeys(ctx, []string{""})
		require.NoError(t, err)
		assert.Nil(t, items)
	})

	t.Run("duplicate_key", func(t *testing.T) {
		items, err := repo.ListActiveJobsByConflictKeys(ctx, []string{"k", "k"})
		require.NoError(t, err)
		assert.Empty(t, items)
	})
}

// ==================== LogRepository edge cases ====================

func TestListByJobIDDefaultLimit(t *testing.T) {
	ctx := context.Background()
	sqlite := newTestSQLite(t)
	logRepo := NewLogRepository(sqlite.DB())

	require.NoError(t, logRepo.Add(ctx, 99, "info", "test", "msg", ""))

	logs, err := logRepo.ListByJobID(ctx, 99, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

// ==================== Closed-DB error tests ====================

func TestJobRepoClosedDBErrors(t *testing.T) {
	db := newClosedTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"UpsertScannedJob", func() error {
			return repo.UpsertScannedJob(ctx, UpsertJobInput{
				FileName: "x.mp4", FileExt: ".mp4", RelPath: "x.mp4",
				AbsPath: "/x.mp4", Number: "X", FileSize: 1,
			})
		}},
		{"GetByID", func() error { _, err := repo.GetByID(ctx, 1); return err }},
		{"UpdateNumber", func() error {
			return repo.UpdateNumber(ctx, 1, "N", "manual", "ok", "high", "")
		}},
		{"UpdateSourcePath", func() error {
			return repo.UpdateSourcePath(ctx, 1, "f.mp4", ".mp4", "f.mp4", "/f.mp4")
		}},
		{"UpdateStatus", func() error {
			_, err := repo.UpdateStatus(ctx, 1, nil, jobdef.StatusFailed, "")
			return err
		}},
		{"MarkDone", func() error { return repo.MarkDone(ctx, 1) }},
		{"SoftDelete", func() error { return repo.SoftDelete(ctx, 1) }},
		{"RecoverProcessingJobs", func() error { return repo.RecoverProcessingJobs(ctx) }},
		{"ListJobs", func() error { _, err := repo.ListJobs(ctx, nil, "", 1, 10); return err }},
		{"ListActiveJobsByConflictKeys", func() error {
			_, err := repo.ListActiveJobsByConflictKeys(ctx, []string{"key"})
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, tc.fn())
		})
	}
}

func TestLogRepoClosedDBErrors(t *testing.T) {
	db := newClosedTestDB(t)
	repo := NewLogRepository(db)
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Add", func() error { return repo.Add(ctx, 1, "info", "s", "m", "") }},
		{"ListByJobID", func() error { _, err := repo.ListByJobID(ctx, 1, 10); return err }},
		{"DeleteByJobID", func() error { return repo.DeleteByJobID(ctx, 1) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, tc.fn())
		})
	}
}

func TestScrapeRepoClosedDBErrors(t *testing.T) {
	db := newClosedTestDB(t)
	repo := NewScrapeDataRepository(db)
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"UpsertRawData", func() error { return repo.UpsertRawData(ctx, 1, "s", "d") }},
		{"GetByJobID", func() error { _, err := repo.GetByJobID(ctx, 1); return err }},
		{"SaveReviewData", func() error { return repo.SaveReviewData(ctx, 1, "d") }},
		{"SaveFinalData", func() error { return repo.SaveFinalData(ctx, 1, "d") }},
		{"DeleteByJobID", func() error { return repo.DeleteByJobID(ctx, 1) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, tc.fn())
		})
	}
}

// ==================== Trigger-based error tests ====================

func TestUpsertScannedJobExecError(t *testing.T) {
	_, db := newTestSQLiteWithRawDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `CREATE TRIGGER block_insert BEFORE INSERT ON yamdc_job_tab
		BEGIN SELECT RAISE(ABORT, 'insert blocked'); END`)
	require.NoError(t, err)

	err = repo.UpsertScannedJob(ctx, UpsertJobInput{
		FileName: "x.mp4", FileExt: ".mp4", RelPath: "x.mp4",
		AbsPath: "/x.mp4", Number: "X", FileSize: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert scanned job failed")
}

func TestUpdateNumberExecError(t *testing.T) {
	_, db := newTestSQLiteWithRawDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()
	id := insertTestJob(t, repo, "A.mp4", "A")

	_, err := db.ExecContext(ctx, `CREATE TRIGGER block_num_update BEFORE UPDATE ON yamdc_job_tab
		WHEN NEW.number != OLD.number
		BEGIN SELECT RAISE(ABORT, 'number update blocked'); END`)
	require.NoError(t, err)

	err = repo.UpdateNumber(ctx, id, "NEW", "manual", "ok", "high", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update job number failed")
}

func TestUpdateSourcePathExecError(t *testing.T) {
	_, db := newTestSQLiteWithRawDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()
	id := insertTestJob(t, repo, "A.mp4", "A")

	_, err := db.ExecContext(ctx, `CREATE TRIGGER block_path_update BEFORE UPDATE ON yamdc_job_tab
		WHEN NEW.file_name != OLD.file_name
		BEGIN SELECT RAISE(ABORT, 'path update blocked'); END`)
	require.NoError(t, err)

	err = repo.UpdateSourcePath(ctx, id, "B.mkv", ".mkv", "B.mkv", "/scan/B.mkv")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update job source path failed")
}

// ==================== Direct function tests ====================

func TestScanJobRowsScanError(t *testing.T) {
	sqlite := newTestSQLite(t)
	ctx := context.Background()

	rows, err := sqlite.DB().QueryContext(ctx, "SELECT 1, 2")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	_, err = scanJobRows(rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan job failed")
}

// ==================== View-based scan/query error tests ====================

func TestListActiveJobsByConflictKeysScanError(t *testing.T) {
	_, db := newTestSQLiteWithRawDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()
	insertTestJob(t, repo, "A.mp4", "A")

	_, err := db.ExecContext(ctx, `ALTER TABLE yamdc_job_tab RENAME TO yamdc_job_tab_bak`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE VIEW yamdc_job_tab AS
		SELECT 'not_an_int' AS id, rel_path, conflict_key, 0 AS deleted_at, status
		FROM yamdc_job_tab_bak`)
	require.NoError(t, err)

	_, err = repo.ListActiveJobsByConflictKeys(ctx, []string{"A.mp4"})
	require.Error(t, err)
}

func TestListJobsQueryError(t *testing.T) {
	_, db := newTestSQLiteWithRawDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()
	insertTestJob(t, repo, "A.mp4", "A")

	_, err := db.ExecContext(ctx, `ALTER TABLE yamdc_job_tab RENAME TO yamdc_job_tab_bak`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE VIEW yamdc_job_tab AS
		SELECT id, 0 AS deleted_at, status FROM yamdc_job_tab_bak`)
	require.NoError(t, err)

	_, err = repo.ListJobs(ctx, nil, "", 1, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list jobs failed")
}

func TestListJobsScanError(t *testing.T) {
	_, db := newTestSQLiteWithRawDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()
	insertTestJob(t, repo, "A.mp4", "A")

	_, err := db.ExecContext(ctx, `ALTER TABLE yamdc_job_tab RENAME TO yamdc_job_tab_bak`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE VIEW yamdc_job_tab AS
		SELECT 'bad' AS id, job_uid, file_name, file_ext, conflict_key,
			rel_path, abs_path, number, raw_number, cleaned_number,
			number_source, number_clean_status, number_clean_confidence,
			number_clean_warnings, file_size, status, error_msg,
			created_at, updated_at, 0 AS deleted_at
		FROM yamdc_job_tab_bak`)
	require.NoError(t, err)

	_, err = repo.ListJobs(ctx, nil, "", 1, 10)
	require.Error(t, err)
}

func TestLogListByJobIDScanError(t *testing.T) {
	_, db := newTestSQLiteWithRawDB(t)
	logRepo := NewLogRepository(db)
	ctx := context.Background()

	require.NoError(t, logRepo.Add(ctx, 1, "info", "test", "msg", ""))

	_, err := db.ExecContext(ctx, `ALTER TABLE yamdc_log_tab RENAME TO yamdc_log_tab_bak`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE VIEW yamdc_log_tab AS
		SELECT 'bad' AS id, job_id, level, stage, message, detail, created_at
		FROM yamdc_log_tab_bak`)
	require.NoError(t, err)

	_, err = logRepo.ListByJobID(ctx, 1, 10)
	require.Error(t, err)
}
