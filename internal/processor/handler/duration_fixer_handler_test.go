package handler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"github.com/xxxsen/yamdc/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestDurationFixer(t *testing.T) {
	tmpVideo := filepath.Join(os.TempDir(), "test_video.mp4")
	defer func() {
		_ = os.RemoveAll(tmpVideo)
	}()
	cmd := exec.Command("ffmpeg", []string{"-f", "lavfi", "-i", "color=c=black:s=320x240:d=10", "-an", "-vcodec", "libx264", tmpVideo, "-y"}...)
	err := cmd.Run()
	assert.NoError(t, err)
	defer filepath.Join(os.TempDir(), "temp_video.mp4")
	fc := &model.FileContext{
		FullFilePath: tmpVideo,
		Meta:         &model.MovieMeta{},
	}
	h, err := CreateHandler(HDurationFixer, nil)
	assert.NoError(t, err)
	err = h.Handle(context.Background(), fc)
	assert.NoError(t, err)
	assert.Equal(t, int64(10), fc.Meta.Duration)
}
