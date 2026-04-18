package store

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCleanupExpirer struct {
	calls int
	err   error
}

func (f *fakeCleanupExpirer) CleanupExpired(context.Context) error {
	f.calls++
	return f.err
}

func TestNewCacheCleanupJobMetadata(t *testing.T) {
	j := NewCacheCleanupJob(&fakeCleanupExpirer{})

	require.NotNil(t, j)
	assert.Equal(t, cacheCleanupJobName, j.Name())
	assert.Equal(t, "@every "+CacheCleanupInterval.String(), j.Spec())
}

func TestNewCacheCleanupJobRunDelegatesToExpirer(t *testing.T) {
	fake := &fakeCleanupExpirer{}
	j := NewCacheCleanupJob(fake)

	require.NoError(t, j.Run(context.Background()))
	assert.Equal(t, 1, fake.calls)
}

func TestNewCacheCleanupJobRunPropagatesError(t *testing.T) {
	errBoom := errors.New("cleanup boom")
	fake := &fakeCleanupExpirer{err: errBoom}
	j := NewCacheCleanupJob(fake)

	err := j.Run(context.Background())

	assert.ErrorIs(t, err, errBoom)
	assert.Equal(t, 1, fake.calls)
}

func TestNewCacheCleanupJobRunNilExpirerIsNoop(t *testing.T) {
	j := NewCacheCleanupJob(nil)

	require.NotNil(t, j)
	assert.NoError(t, j.Run(context.Background()))
}
