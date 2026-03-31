package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	"github.com/xxxsen/yamdc/internal/repository"
)

func TestScanCleansMissingInitAndFailedJobsAndMarksReviewingMissing(t *testing.T) {
	ctx := context.Background()
	scanDir := t.TempDir()
	sqlite, err := repository.NewSQLite(filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})

	repo := repository.NewJobRepository(sqlite.DB())
	svc := New(scanDir, nil, repo, numbercleaner.NewPassthroughCleaner())

	liveFile := filepath.Join(scanDir, "LIVE-001.mp4")
	require.NoError(t, os.WriteFile(liveFile, []byte("x"), 0644))

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
	require.NoError(t, err)
	require.Nil(t, staleJob)

	reviewJob, err := repo.GetByID(ctx, reviewID)
	require.NoError(t, err)
	require.NotNil(t, reviewJob)
	require.Equal(t, jobdef.StatusFailed, reviewJob.Status)
	require.Contains(t, reviewJob.ErrorMsg, "source file missing")
}
