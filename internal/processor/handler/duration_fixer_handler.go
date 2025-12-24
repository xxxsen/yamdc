package handler

import (
	"context"
	"github.com/xxxsen/yamdc/internal/ffmpeg"
	"github.com/xxxsen/yamdc/internal/model"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type durationFixerHandler struct {
}

func (h *durationFixerHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	if fc.Meta.Duration > 0 {
		return nil
	}
	if !ffmpeg.IsFFProbeEnabled() {
		return nil
	}
	duration, err := ffmpeg.ReadDuration(ctx, fc.FullFilePath)
	if err != nil {
		return err
	}
	fc.Meta.Duration = int64(duration)
	logutil.GetLogger(ctx).Debug("rewrite video duration succ", zap.Float64("duration", duration))
	return nil
}

func init() {
	Register(HDurationFixer, HandlerToCreator(&durationFixerHandler{}))
}
