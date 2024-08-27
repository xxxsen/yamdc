package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiskStorage(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "storage")
	st := NewDiskStorage(dir)
	ctx := context.Background()
	key := "aaa"
	value := []byte("hello world")
	err := st.PutData(ctx, key, value)
	assert.NoError(t, err)
	exist, err := st.IsDataExist(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exist)
	data, err := st.GetData(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, data)

	//check not exist
	exist, err = st.IsDataExist(ctx, "hello")
	assert.NoError(t, err)
	assert.False(t, exist)
}
