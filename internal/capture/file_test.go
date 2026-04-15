package capture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoveFile(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (src, dst string)
		wantErr bool
	}{
		{
			name: "same device rename",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.mp4")
				dst := filepath.Join(dir, "dst.mp4")
				require.NoError(t, os.WriteFile(src, []byte("content"), 0o644))
				return src, dst
			},
		},
		{
			name: "different directory",
			setup: func(t *testing.T) (string, string) {
				srcDir := t.TempDir()
				dstDir := t.TempDir()
				src := filepath.Join(srcDir, "src.mp4")
				dst := filepath.Join(dstDir, "dst.mp4")
				require.NoError(t, os.WriteFile(src, []byte("content"), 0o644))
				return src, dst
			},
		},
		{
			name: "source not exist",
			setup: func(t *testing.T) (string, string) {
				return filepath.Join(t.TempDir(), "nonexist.mp4"), filepath.Join(t.TempDir(), "dst.mp4")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst := tt.setup(t)
			err := moveFile(src, dst)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.FileExists(t, dst)
				data, err := os.ReadFile(dst)
				require.NoError(t, err)
				assert.Equal(t, "content", string(data))
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (src, dst string)
		wantErr bool
	}{
		{
			name: "normal copy",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt")
				dst := filepath.Join(dir, "dst.txt")
				require.NoError(t, os.WriteFile(src, []byte("hello"), 0o644))
				return src, dst
			},
		},
		{
			name: "source does not exist",
			setup: func(t *testing.T) (string, string) {
				return filepath.Join(t.TempDir(), "nonexist.txt"), filepath.Join(t.TempDir(), "dst.txt")
			},
			wantErr: true,
		},
		{
			name: "destination dir does not exist",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				src := filepath.Join(dir, "src.txt")
				require.NoError(t, os.WriteFile(src, []byte("hello"), 0o644))
				return src, filepath.Join(dir, "nonexist_dir", "dst.txt")
			},
			wantErr: true,
		},
		{
			name: "empty file",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				src := filepath.Join(dir, "empty.txt")
				dst := filepath.Join(dir, "dst.txt")
				require.NoError(t, os.WriteFile(src, []byte{}, 0o644))
				return src, dst
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst := tt.setup(t)
			err := copyFile(src, dst)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.FileExists(t, dst)
				srcData, err := os.ReadFile(src)
				require.NoError(t, err)
				dstData, err := os.ReadFile(dst)
				require.NoError(t, err)
				assert.Equal(t, srcData, dstData)
			}
		})
	}
}

func TestMoveCrossDevice(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		src := filepath.Join(srcDir, "source.mp4")
		dst := filepath.Join(dstDir, "dest.mp4")
		require.NoError(t, os.WriteFile(src, []byte("video data"), 0o644))

		err := moveCrossDevice(src, dst)
		require.NoError(t, err)
		assert.FileExists(t, dst)
		_, err = os.Stat(src)
		assert.True(t, os.IsNotExist(err))
		data, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, "video data", string(data))
	})

	t.Run("copy fails - source not found", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "nonexist.mp4")
		dst := filepath.Join(t.TempDir(), "dest.mp4")
		err := moveCrossDevice(src, dst)
		assert.Error(t, err)
	})
}
