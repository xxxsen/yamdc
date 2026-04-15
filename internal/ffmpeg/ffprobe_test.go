package ffmpeg

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsFFProbeEnabled(_ *testing.T) {
	_ = IsFFProbeEnabled()
}

func TestNewFFProbe_LookPathError(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(string) (string, error) {
		return "", errors.New("missing")
	}
	_, err := NewFFProbe()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ffprobe")
}

func TestFFProbe_ReadDuration_OutputError(t *testing.T) {
	origCC := commandContext
	t.Cleanup(func() { commandContext = origCC })
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}
	p := &FFProbe{cmd: "/x"}
	_, err := p.ReadDuration(context.Background(), "/tmp/nope.mp4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "call ffprobe")
}

func TestFFProbe_ReadDuration_ParseError(t *testing.T) {
	origCC := commandContext
	t.Cleanup(func() { commandContext = origCC })
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `printf 'not-a-number'`)
	}
	p := &FFProbe{cmd: "/x"}
	_, err := p.ReadDuration(context.Background(), "/any")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse video duration")
}

func TestFFProbe_ReadDuration_Success(t *testing.T) {
	origCC := commandContext
	t.Cleanup(func() { commandContext = origCC })
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `printf '  12.5 \n'`)
	}
	p := &FFProbe{cmd: "/x"}
	d, err := p.ReadDuration(context.Background(), "/any")
	require.NoError(t, err)
	assert.InDelta(t, 12.5, d, 0.001)
}

func TestReadDuration_NotEnabled(t *testing.T) {
	saved := defaultFFProbe
	defaultFFProbe = nil
	t.Cleanup(func() { defaultFFProbe = saved })

	_, err := ReadDuration(context.Background(), "/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestReadDuration_ForwardWhenEnabled(t *testing.T) {
	if defaultFFProbe == nil {
		t.Skip("ffprobe not on PATH at init")
	}
	origCC := commandContext
	t.Cleanup(func() { commandContext = origCC })
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `printf '3'`)
	}

	d, err := ReadDuration(context.Background(), "/ignored")
	require.NoError(t, err)
	assert.InDelta(t, 3.0, d, 0.001)
}

func TestFFProbe_ReadDuration_Integration(t *testing.T) {
	if _, err := lookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
	origCC := commandContext
	commandContext = exec.CommandContext
	t.Cleanup(func() { commandContext = origCC })

	dir := t.TempDir()
	vpath := filepath.Join(dir, "stub.mp4")
	// Minimal MP4 header-ish bytes; ffprobe may still error — use ffmpeg to mux if needed.
	// Prefer: generate with ffmpeg when both exist.
	if _, err := lookPath("ffmpeg"); err == nil {
		gen := exec.CommandContext(context.Background(), "ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono", "-f", "lavfi", "-i",
			"color=c=black:s=64x64:d=1", "-shortest", "-c:v", "libx264", "-t", "1", vpath)
		if err := gen.Run(); err != nil {
			t.Skip("could not generate test video:", err)
		}
	} else {
		t.Skip("ffmpeg not available to generate test video")
	}

	inst, err := NewFFProbe()
	require.NoError(t, err)
	dur, err := inst.ReadDuration(context.Background(), vpath)
	require.NoError(t, err)
	assert.Greater(t, dur, 0.0)
}
