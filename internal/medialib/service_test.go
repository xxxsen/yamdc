package medialib

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/repository"
)

func newTestMediaService(t *testing.T) *Service {
	t.Helper()
	sqlite, err := repository.NewSQLite(filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlite.Close())
	})
	return NewService(sqlite.DB(), t.TempDir(), t.TempDir())
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
