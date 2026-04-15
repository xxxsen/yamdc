package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/ffmpeg"
	"github.com/xxxsen/yamdc/internal/model"
)

func TestDurationFixerHandlerSkipsWhenDurationSet(t *testing.T) {
	h := &durationFixerHandler{}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{Duration: 3600},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
	assert.Equal(t, int64(3600), fc.Meta.Duration)
}

func TestDurationFixerHandlerSkipsWhenFFProbeDisabled(t *testing.T) {
	if ffmpeg.IsFFProbeEnabled() {
		t.Skip("ffprobe is available, skipping disabled-path test")
	}
	h := &durationFixerHandler{}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{Duration: 0},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), fc.Meta.Duration)
}

func TestDurationFixerHandlerWithFFProbe(t *testing.T) {
	if !ffmpeg.IsFFProbeEnabled() {
		t.Skip("ffprobe not available")
	}

	h := &durationFixerHandler{}

	t.Run("invalid file path", func(t *testing.T) {
		fc := &model.FileContext{
			Meta:         &model.MovieMeta{Duration: 0},
			FullFilePath: "/nonexistent/file.mp4",
		}
		err := h.Handle(context.Background(), fc)
		assert.Error(t, err)
	})

	t.Run("invalid media file", func(t *testing.T) {
		tmpDir := t.TempDir()
		fakeFile := filepath.Join(tmpDir, "fake.mp4")
		require.NoError(t, os.WriteFile(fakeFile, []byte("not a real video"), 0o600))
		fc := &model.FileContext{
			Meta:         &model.MovieMeta{Duration: 0},
			FullFilePath: fakeFile,
		}
		err := h.Handle(context.Background(), fc)
		assert.Error(t, err)
	})
}
