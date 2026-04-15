package medialib

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/common/logger"
	"github.com/xxxsen/yamdc/internal/repository"
)

func newTestMediaService(t *testing.T) *Service {
	t.Helper()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})
	return NewService(sqlite.DB(), t.TempDir(), t.TempDir())
}

func withCapturedLogs(t *testing.T) (string, func()) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "app.log")
	lg := logger.Init(logPath, "debug", 1, 1024*1024, 1, false)
	return logPath, func() {
		_ = lg.Sync()
		logger.Init("", "debug", 0, 0, 0, true)
	}
}

func TestServiceStartRecoversRunningTaskStates(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.saveTaskState(ctx, TaskState{
		TaskKey:   TaskSync,
		Status:    "running",
		Total:     10,
		Processed: 3,
		Message:   "同步媒体库中",
		StartedAt: 123,
		UpdatedAt: 456,
	}))
	require.NoError(t, svc.saveTaskState(ctx, TaskState{
		TaskKey:   TaskMove,
		Status:    "running",
		Total:     5,
		Processed: 2,
		Message:   "移动到媒体库中",
		StartedAt: 789,
		UpdatedAt: 999,
	}))

	svc.Start(ctx)

	syncState, err := svc.getTaskState(ctx, TaskSync)
	require.NoError(t, err)
	require.Equal(t, "failed", syncState.Status)
	require.Equal(t, "server restarted while task running", syncState.Message)
	require.NotZero(t, syncState.FinishedAt)

	moveState, err := svc.getTaskState(ctx, TaskMove)
	require.NoError(t, err)
	require.Equal(t, "failed", moveState.Status)
	require.Equal(t, "server restarted while task running", moveState.Message)
	require.NotZero(t, moveState.FinishedAt)
}

func TestServiceStartKeepsNonRunningTaskState(t *testing.T) {
	svc := newTestMediaService(t)
	ctx := context.Background()

	require.NoError(t, svc.saveTaskState(ctx, TaskState{
		TaskKey:    TaskSync,
		Status:     "completed",
		Message:    "ok",
		StartedAt:  100,
		FinishedAt: 200,
		UpdatedAt:  300,
	}))

	svc.Start(ctx)

	state, err := svc.getTaskState(ctx, TaskSync)
	require.NoError(t, err)
	require.Equal(t, "completed", state.Status)
	require.Equal(t, "ok", state.Message)
	require.Equal(t, int64(200), state.FinishedAt)
}

func TestRunFullSyncLogsSyncedMediaMetadata(t *testing.T) {
	logPath, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	defer func() { require.NoError(t, sqlite.Close()) }()

	itemDir := filepath.Join(libraryDir, "movie")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.nfo"), []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes" ?>
<movie>
  <title>Sample Title</title>
  <originaltitle>Sample Original</originaltitle>
  <id>ABC-123</id>
  <premiered>2024-01-02</premiered>
</movie>`), 0o600))

	svc := NewService(sqlite.DB(), libraryDir, saveDir)
	require.NoError(t, svc.runFullSync(context.Background(), "manual"))

	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logs := string(raw)
	require.Contains(t, logs, "media library sync item started")
	require.Contains(t, logs, "media library detail synced")
	require.Contains(t, logs, "rel_path")
	require.Contains(t, logs, "movie")
	require.Contains(t, logs, "ABC-123")
	require.Contains(t, logs, "Sample Title")
}

func TestRunMoveLogsPerItemProgress(t *testing.T) {
	logPath, cleanup := withCapturedLogs(t)
	defer cleanup()

	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	defer func() { require.NoError(t, sqlite.Close()) }()

	itemDir := filepath.Join(saveDir, "movie")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.nfo"), []byte(`<movie><title>Moved</title><id>XYZ-987</id></movie>`), 0o600))

	svc := NewService(sqlite.DB(), libraryDir, saveDir)
	require.NoError(t, svc.runMove(context.Background()))

	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logs := string(raw)
	require.Contains(t, logs, "move to media library item started")
	require.Contains(t, logs, "move to media library item finished")
	require.Contains(t, logs, "move to media library completed")
	require.True(t, strings.Contains(logs, "movie"))
}
