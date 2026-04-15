package ffmpeg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

var errFFMpegNotEnabled = errors.New("ffmpeg is not enabled")

var defaultFFMpeg *FFMpeg

func init() {
	inst, err := NewFFMpeg()
	if err != nil {
		return
	}
	defaultFFMpeg = inst
}

func IsFFMpegEnabled() bool {
	return defaultFFMpeg != nil
}

type FFMpeg struct {
	cmd string
}

// lookPath resolves the ffmpeg/ffprobe binary (overridable in tests).
var lookPath = exec.LookPath

// commandContext builds exec.Cmd instances (overridable in tests).
var commandContext = exec.CommandContext

func NewFFMpeg() (*FFMpeg, error) {
	location, err := lookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}
	return &FFMpeg{cmd: location}, nil
}

func (p *FFMpeg) ConvertToYuv420pJpegFromBytes(ctx context.Context, data []byte) ([]byte, error) {
	dstFile := filepath.Join(os.TempDir(), "image-conv-dst-"+uuid.New().String())
	defer func() {
		_ = os.Remove(dstFile)
	}()
	cmd := commandContext(ctx,
		p.cmd,
		"-i",
		"pipe:0",
		"-vf",
		"format=yuv420p",
		"-f",
		"image2",
		dstFile,
	)
	cmd.Stdin = bytes.NewReader(data)
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("call ffmpeg to conv failed, err:%w", err)
	}
	data, err = os.ReadFile(dstFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read converted data, err:%w", err)
	}
	return data, nil
}

func ConvertToYuv420pJpegFromBytes(ctx context.Context, data []byte) ([]byte, error) {
	if defaultFFMpeg == nil {
		return nil, errFFMpegNotEnabled
	}
	return defaultFFMpeg.ConvertToYuv420pJpegFromBytes(ctx, data)
}
