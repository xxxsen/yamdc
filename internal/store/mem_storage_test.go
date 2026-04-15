package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStorage_GetPutIsExist(t *testing.T) {
	s := NewMemStorage()
	ctx := context.Background()

	_, err := s.GetData(ctx, "x")
	require.Error(t, err)

	ok, err := s.IsDataExist(ctx, "x")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.PutData(ctx, "x", []byte("hello"), 0))

	v, err := s.GetData(ctx, "x")
	require.NoError(t, err)
	assert.Equal(t, "hello", string(v))

	ok, err = s.IsDataExist(ctx, "x")
	require.NoError(t, err)
	assert.True(t, ok)
}
