package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/repository"
)

type blockingCleaner struct {
	startOnce sync.Once
	started   chan struct{}
	release   chan struct{}
}

func (c *blockingCleaner) Clean(input string) (*movieidcleaner.Result, error) {
	c.startOnce.Do(func() {
		close(c.started)
	})
	<-c.release
	return movieidcleaner.NewPassthroughCleaner().Clean(input)
}

func (c *blockingCleaner) Explain(input string) (*movieidcleaner.ExplainResult, error) {
	return movieidcleaner.NewPassthroughCleaner().Explain(input)
}

type erroringCleaner struct {
	err error
}

func (c *erroringCleaner) Clean(string) (*movieidcleaner.Result, error) {
	return nil, c.err
}

func (c *erroringCleaner) Explain(string) (*movieidcleaner.ExplainResult, error) {
	return nil, nil //nolint:nilnil // 测试桩显式返回 (nil, nil)
}

type nilResultCleaner struct{}

func (nilResultCleaner) Clean(string) (*movieidcleaner.Result, error) {
	return nil, nil //nolint:nilnil // 测试桩显式返回 (nil, nil)
}

func (nilResultCleaner) Explain(string) (*movieidcleaner.ExplainResult, error) {
	return nil, nil //nolint:nilnil // 测试桩显式返回 (nil, nil)
}

type emptyNormalizedCleaner struct{}

func (emptyNormalizedCleaner) Clean(input string) (*movieidcleaner.Result, error) {
	return &movieidcleaner.Result{
		RawInput:   input,
		Normalized: "",
		Status:     movieidcleaner.StatusSuccess,
		Confidence: movieidcleaner.ConfidenceHigh,
		Warnings:   []string{"w1", "w2"},
	}, nil
}

func (emptyNormalizedCleaner) Explain(string) (*movieidcleaner.ExplainResult, error) {
	return nil, nil //nolint:nilnil // 测试桩显式返回 (nil, nil)
}

type normalizedCleaner struct {
	out string
}

func (c normalizedCleaner) Clean(string) (*movieidcleaner.Result, error) {
	return &movieidcleaner.Result{
		Normalized: c.out,
		Status:     movieidcleaner.StatusSuccess,
		Confidence: movieidcleaner.ConfidenceMedium,
	}, nil
}

func (normalizedCleaner) Explain(string) (*movieidcleaner.ExplainResult, error) {
	return nil, nil //nolint:nilnil // 测试桩显式返回 (nil, nil)
}

func TestScanCleansMissingInitAndFailedJobsAndMarksReviewingMissing(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, movieidcleaner.NewPassthroughCleaner())

	liveFile := filepath.Join(scanDir, "LIVE-001.mp4")
	require.NoError(t, os.WriteFile(liveFile, []byte("x"), 0o600))

	require.NoError(t, repo.UpsertScannedJob(ctx, repository.UpsertJobInput{
		FileName: "STALE-001.mp4",
		FileExt:  ".mp4",
		RelPath:  "STALE-001.mp4",
		AbsPath:  filepath.Join(scanDir, "STALE-001.mp4"),
		Number:   "STALE-001",
		FileSize: 1,
	}))
	require.NoError(t, repo.UpsertScannedJob(ctx, repository.UpsertJobInput{
		FileName: "REVIEW-001.mp4",
		FileExt:  ".mp4",
		RelPath:  "REVIEW-001.mp4",
		AbsPath:  filepath.Join(scanDir, "REVIEW-001.mp4"),
		Number:   "REVIEW-001",
		FileSize: 1,
	}))

	result, err := repo.ListJobs(ctx, nil, "", 1, 0)
	require.NoError(t, err)
	require.Len(t, result.Items, 2)

	var staleID int64
	var reviewID int64
	for _, item := range result.Items {
		switch item.FileName {
		case "STALE-001.mp4":
			staleID = item.ID
		case "REVIEW-001.mp4":
			reviewID = item.ID
		}
	}
	require.NotZero(t, staleID)
	require.NotZero(t, reviewID)

	ok, err := repo.UpdateStatus(ctx, reviewID, []jobdef.Status{jobdef.StatusInit}, jobdef.StatusReviewing, "")
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, svc.Scan(ctx))

	liveJobs, err := repo.ListJobs(ctx, nil, "", 1, 0)
	require.NoError(t, err)
	require.Len(t, liveJobs.Items, 2)

	staleJob, err := repo.GetByID(ctx, staleID)
	require.ErrorIs(t, err, repository.ErrJobNotFound)
	require.Nil(t, staleJob)

	reviewJob, err := repo.GetByID(ctx, reviewID)
	require.NoError(t, err)
	require.NotNil(t, reviewJob)
	require.Equal(t, jobdef.StatusFailed, reviewJob.Status)
	require.Contains(t, reviewJob.ErrorMsg, "source file missing")
}

func TestScanRejectsReentryWhileRunning(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	filePath := filepath.Join(scanDir, "HEYZO-0040.mp4")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))

	cleaner := &blockingCleaner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, cleaner)

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- svc.Scan(ctx)
	}()

	<-cleaner.started

	err = svc.Scan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already running")

	close(cleaner.release)
	require.NoError(t, <-firstDone)
}

func TestNewRegistersExtraMediaExtensionsCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, []string{".iso", ".ISO"}, repo, movieidcleaner.NewPassthroughCleaner())

	lower := filepath.Join(scanDir, "disc.iso")
	upper := filepath.Join(scanDir, "DISC.ISO")
	require.NoError(t, os.WriteFile(lower, []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(upper, []byte("b"), 0o600))

	require.NoError(t, svc.Scan(ctx))

	jobs, err := repo.ListJobs(ctx, nil, "", 1, 0)
	require.NoError(t, err)
	require.Len(t, jobs.Items, 2)
}

func TestScanSkipsNonMediaFiles(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "notes.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "movie.mp4"), []byte("x"), 0o600))

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, nil)

	require.NoError(t, svc.Scan(ctx))

	jobs, err := repo.ListJobs(ctx, nil, "", 1, 0)
	require.NoError(t, err)
	require.Len(t, jobs.Items, 1)
	require.Equal(t, "movie.mp4", jobs.Items[0].FileName)
}

func TestScanNonexistentScanDir(t *testing.T) {
	ctx := context.Background()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	missing := filepath.Join(t.TempDir(), "missing-scan-root-404")
	svc := New(missing, nil, repo, nil)

	err = svc.Scan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "walk scan dir")
}

func TestScanUpsertCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scanDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "ABC-123.mp4"), []byte("x"), 0o600))

	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, nil)

	err = svc.Scan(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestScanUpsertFailsWhenDBClosed(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "ABC-123.mp4"), []byte("x"), 0o600))

	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, nil)

	require.NoError(t, sqlite.Close())

	err = svc.Scan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "upsert scanned job")
}

func TestScanCleanupListJobsFailsWhenDBClosed(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()

	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, nil)

	require.NoError(t, sqlite.Close())

	err = svc.Scan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "list jobs")
}

func TestScanCleanerErrorPropagates(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "BAD.mp4"), []byte("x"), 0o600))

	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, &erroringCleaner{err: errors.New("clean failed")})

	err = svc.Scan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clean scan number")
}

func TestScanRecordsNilCleanerAndNilCleanerResultPaths(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "RAW-001.mp4"), []byte("x"), 0o600))

	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, nilResultCleaner{})

	require.NoError(t, svc.Scan(ctx))

	jobs, err := repo.ListJobs(ctx, nil, "", 1, 0)
	require.NoError(t, err)
	require.Len(t, jobs.Items, 1)
	require.Equal(t, "RAW-001", jobs.Items[0].Number)
	require.Equal(t, "raw", jobs.Items[0].NumberSource)
}

func TestScanEmptyNormalizedCleanerKeepsRawNumberAndJoinsWarnings(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "TAGGED-001.mp4"), []byte("x"), 0o600))

	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, emptyNormalizedCleaner{})

	require.NoError(t, svc.Scan(ctx))

	jobs, err := repo.ListJobs(ctx, nil, "", 1, 0)
	require.NoError(t, err)
	require.Len(t, jobs.Items, 1)
	require.Equal(t, "TAGGED-001", jobs.Items[0].Number)
	require.Equal(t, "raw", jobs.Items[0].NumberSource)
	require.Equal(t, "w1; w2", jobs.Items[0].NumberCleanWarnings)
}

func TestScanNormalizedCleanerOverridesNumber(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "messy.mp4"), []byte("x"), 0o600))

	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, normalizedCleaner{out: "SSIS-001"})

	require.NoError(t, svc.Scan(ctx))

	jobs, err := repo.ListJobs(ctx, nil, "", 1, 0)
	require.NoError(t, err)
	require.Len(t, jobs.Items, 1)
	require.Equal(t, "SSIS-001", jobs.Items[0].Number)
	require.Equal(t, "cleaner", jobs.Items[0].NumberSource)
}

func TestScanWalkReadErrorOnUnreadableSubdir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("chmod-based permission tests are not meaningful as root")
	}

	ctx := context.Background()
	scanDir := t.TempDir()
	sub := filepath.Join(scanDir, "locked")
	require.NoError(t, os.Mkdir(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "hidden.mp4"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(sub, 0))
	t.Cleanup(func() {
		_ = os.Chmod(sub, 0o755)
	})

	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, nil)

	err = svc.Scan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "walk scan dir")
}
