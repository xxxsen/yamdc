package handler

import (
	"av-capture/model"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type durationFixer struct {
	once        sync.Once
	ffprobePath string
}

func (h *durationFixer) ffprobe() (string, bool) {
	h.once.Do(func() {
		path, err := exec.LookPath("ffprobe")
		if err != nil {
			logutil.GetLogger(context.Background()).Error("search ffprobe cmd failed", zap.Error(err))
		}
		h.ffprobePath = path
	})

	if len(h.ffprobePath) == 0 {
		return "", false
	}
	return h.ffprobePath, true
}

func (h *durationFixer) Handle(ctx context.Context, fc *model.FileContext) error {
	if fc.Meta.Duration > 0 {
		return nil
	}
	ffprobePath, ok := h.ffprobe()
	if !ok {
		logutil.GetLogger(ctx).Error("unable to find ffprobe in sys path, skip fix duration")
		return nil
	}
	cmd := exec.CommandContext(ctx, ffprobePath, []string{"-i", fc.FullFilePath, "-show_entries", "format=duration", "-v", "quiet", "-of", "csv=p=0"}...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("call ffprobe to detect video duration failed, err:%w", err)
	}
	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return fmt.Errorf("parse video duration failed, duration:%s, err:%w", durationStr, err)
	}
	fc.Meta.Duration = int64(duration)
	logutil.GetLogger(ctx).Debug("rewrite video duration succ", zap.Float64("duration", duration))
	return nil
}

func init() {
	Register(HDurationFixer, HandlerToCreator(&durationFixer{}))
}
