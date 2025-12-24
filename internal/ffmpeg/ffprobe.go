package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var defaultFFProbe *FFProbe

func init() {
	inst, err := NewFFProbe()
	if err != nil {
		return
	}
	defaultFFProbe = inst
}

func IsFFProbeEnabled() bool {
	return defaultFFProbe != nil
}

type FFProbe struct {
	cmd string
}

func NewFFProbe() (*FFProbe, error) {
	location, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("search ffprobe command failed, err:%w", err)
	}
	return &FFProbe{cmd: location}, nil
}

func (p *FFProbe) ReadDuration(ctx context.Context, file string) (float64, error) {
	cmd := exec.CommandContext(ctx, p.cmd, []string{"-i", file, "-show_entries", "format=duration", "-v", "quiet", "-of", "csv=p=0"}...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("call ffprobe to detect video duration failed, err:%w", err)
	}
	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse video duration failed, duration:%s, err:%w", durationStr, err)
	}
	return duration, nil
}

func ReadDuration(ctx context.Context, file string) (float64, error) {
	return defaultFFProbe.ReadDuration(ctx, file)
}
