package ffmpeg

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsFFMpegEnabled(_ *testing.T) {
	_ = IsFFMpegEnabled()
}

func TestNewFFMpeg_LookPathError(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(string) (string, error) {
		return "", errors.New("not in path")
	}
	_, err := NewFFMpeg()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ffmpeg not found")
}

func TestFFMpeg_Convert_RunError(t *testing.T) {
	origCC := commandContext
	t.Cleanup(func() { commandContext = origCC })
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}
	p := &FFMpeg{cmd: "/nonexistent-ffmpeg"}
	_, err := p.ConvertToYuv420pJpegFromBytes(context.Background(), []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "call ffmpeg to conv failed")
}

func TestFFMpeg_Convert_ReadOutputError(t *testing.T) {
	origCC := commandContext
	t.Cleanup(func() { commandContext = origCC })
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}
	p := &FFMpeg{cmd: "/x"}
	_, err := p.ConvertToYuv420pJpegFromBytes(context.Background(), []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read converted data")
}

func TestFFMpeg_Convert_Success_Mock(t *testing.T) {
	origCC := commandContext
	t.Cleanup(func() { commandContext = origCC })

	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture")
	require.NoError(t, os.WriteFile(fixture, []byte("out"), 0o600))

	commandContext = func(ctx context.Context, _ string, arg ...string) *exec.Cmd {
		dst := arg[len(arg)-1]
		return exec.CommandContext(ctx, "cp", fixture, dst)
	}
	p := &FFMpeg{cmd: "/x"}
	out, err := p.ConvertToYuv420pJpegFromBytes(context.Background(), []byte("in"))
	require.NoError(t, err)
	assert.Equal(t, []byte("out"), out)
}

func TestConvertToYuv420pJpegFromBytes_NotEnabled(t *testing.T) {
	saved := defaultFFMpeg
	defaultFFMpeg = nil
	t.Cleanup(func() { defaultFFMpeg = saved })

	_, err := ConvertToYuv420pJpegFromBytes(context.Background(), []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestConvertToYuv420pJpegFromBytes_ForwardWhenEnabled(t *testing.T) {
	if defaultFFMpeg == nil {
		t.Skip("ffmpeg not on PATH at init")
	}
	origCC := commandContext
	t.Cleanup(func() { commandContext = origCC })

	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture")
	require.NoError(t, os.WriteFile(fixture, []byte("ok"), 0o600))

	commandContext = func(ctx context.Context, _ string, arg ...string) *exec.Cmd {
		dst := arg[len(arg)-1]
		return exec.CommandContext(ctx, "cp", fixture, dst)
	}

	out, err := ConvertToYuv420pJpegFromBytes(context.Background(), []byte("in"))
	require.NoError(t, err)
	assert.Equal(t, []byte("ok"), out)
}

func TestFFMpeg_Convert_Integration_RealFfmpeg(t *testing.T) {
	if _, err := lookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	origCC := commandContext
	commandContext = exec.CommandContext
	t.Cleanup(func() { commandContext = origCC })

	inst, err := NewFFMpeg()
	require.NoError(t, err)

	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: byte(x * 40), G: byte(y * 40), B: 0x80, A: 0xff})
		}
	}
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}))

	out, err := inst.ConvertToYuv420pJpegFromBytes(context.Background(), buf.Bytes())
	require.NoError(t, err)
	require.NotEmpty(t, out)
	_, format, err := image.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
}
