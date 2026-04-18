package bundle

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteSyncJobLocalReturnsNil(t *testing.T) {
	m, err := NewManager("name", t.TempDir(), nil, SourceTypeLocal, t.TempDir(), "cache",
		func(_ context.Context, _ *Data) error { return nil })
	require.NoError(t, err)

	assert.Nil(t, m.RemoteSyncJob("prefix"), "local bundle should not produce a cron job")
}

func TestRemoteSyncJobNilReceiverReturnsNil(t *testing.T) {
	var m *Manager
	assert.Nil(t, m.RemoteSyncJob("prefix"))
}

func TestRemoteSyncJobMetadata(t *testing.T) {
	m := &Manager{
		name:         "ruleset",
		sourceType:   SourceTypeRemote,
		syncInterval: 24 * time.Hour,
	}

	job := m.RemoteSyncJob("movieid_cleaner")
	require.NotNil(t, job)
	assert.Equal(t, "movieid_cleaner_ruleset_remote_sync", job.Name())
	assert.Equal(t, "@every 24h0m0s", job.Spec())
}

func TestRemoteSyncJobRunWrapsSyncError(t *testing.T) {
	m := &Manager{
		name:         "ruleset",
		sourceType:   SourceTypeRemote,
		location:     "https://github.com/owner/repo",
		cacheDir:     t.TempDir(),
		syncInterval: time.Hour,
		cli: &mockHTTPClient{
			doFunc: func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("remote unreachable")
			},
		},
		cb: func(_ context.Context, _ *Data) error { return nil },
	}

	job := m.RemoteSyncJob("prefix")
	require.NotNil(t, job)

	err := job.Run(context.Background())
	require.Error(t, err, "sync failure should surface to caller; cronscheduler adapter will log as warn")
	assert.Contains(t, err.Error(), "prefix_ruleset_remote_sync")
}
