package infra

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/store"
)

func TestBuildCacheStoreUsesPebble(t *testing.T) {
	s, err := BuildCacheStore(context.Background(), t.TempDir())
	require.NoError(t, err)
	closer, ok := s.(io.Closer)
	require.True(t, ok)
	t.Cleanup(func() {
		require.NoError(t, closer.Close())
	})
	_, ok = s.(store.CacheCleanupExpirer)
	assert.True(t, ok)

	require.NoError(t, s.PutData(context.Background(), "k", []byte("value"), 0))
	got, err := s.GetData(context.Background(), "k")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), got)
}
