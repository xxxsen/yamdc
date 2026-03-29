package job

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
)

func newTestService(t *testing.T) (*Service, *repository.JobRepository) {
	t.Helper()
	sqlite, err := repository.NewSQLite(filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	jobRepo := repository.NewJobRepository(sqlite.DB())
	logRepo := repository.NewLogRepository(sqlite.DB())
	scrapeRepo := repository.NewScrapeDataRepository(sqlite.DB())
	return NewService(jobRepo, logRepo, scrapeRepo, nil, store.NewMemStorage()), jobRepo
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
	require.NoError(t, os.WriteFile(file, []byte("x"), 0644))
	jobID := insertJob(t, repo, file, jobdef.StatusFailed)

	err := svc.Delete(context.Background(), jobID)
	require.NoError(t, err)

	_, statErr := os.Stat(file)
	require.True(t, os.IsNotExist(statErr))

	got, err := repo.GetByID(context.Background(), jobID)
	require.NoError(t, err)
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

func TestServiceResolveJobSourcePathFallsBackToRenamedNumberFile(t *testing.T) {
	svc, repo := newTestService(t)
	dir := t.TempDir()
	newFile := filepath.Join(dir, "HEYZO-0040.mp4")
	require.NoError(t, os.WriteFile(newFile, []byte("x"), 0644))

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

func TestServiceGetJobConflictDetectsDuplicateTargetFileName(t *testing.T) {
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

	conflict1, err := svc.GetJobConflict(context.Background(), job1)
	require.NoError(t, err)
	require.NotNil(t, conflict1)
	require.Contains(t, conflict1.Reason, "冲突")

	conflict2, err := svc.GetJobConflict(context.Background(), job2)
	require.NoError(t, err)
	require.NotNil(t, conflict2)
	require.Contains(t, conflict2.Reason, "冲突")
}

func TestServiceGetJobConflictAllowsMultiCDTargets(t *testing.T) {
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

	conflict1, err := svc.GetJobConflict(context.Background(), job1)
	require.NoError(t, err)
	require.Nil(t, conflict1)

	conflict2, err := svc.GetJobConflict(context.Background(), job2)
	require.NoError(t, err)
	require.Nil(t, conflict2)
}

func TestServiceGetJobConflictAllowsSpecialSuffixTargets(t *testing.T) {
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

	conflict1, err := svc.GetJobConflict(context.Background(), job1)
	require.NoError(t, err)
	require.Nil(t, conflict1)

	conflict2, err := svc.GetJobConflict(context.Background(), job2)
	require.NoError(t, err)
	require.Nil(t, conflict2)
}

func TestServiceGetJobConflictIgnoresExistingSavedirTarget(t *testing.T) {
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

	conflict, err := svc.GetJobConflict(context.Background(), jobItem)
	require.NoError(t, err)
	require.Nil(t, conflict)
}
