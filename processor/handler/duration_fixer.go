package handler

import (
	"context"
	"yamdc/ffmpeg"
	"yamdc/model"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type durationFixer struct {
	ffprobeInst *ffmpeg.FFProbe
}

func (h *durationFixer) Handle(ctx context.Context, fc *model.FileContext) error {
	if fc.Meta.Duration > 0 {
		return nil
	}
	if h.ffprobeInst == nil {
		return nil
	}
	duration, err := h.ffprobeInst.ReadDuration(ctx, fc.FullFilePath)
	if err != nil {
		return err
	}
	fc.Meta.Duration = int64(duration)
	logutil.GetLogger(ctx).Debug("rewrite video duration succ", zap.Float64("duration", duration))
	return nil
}

func createDurationFixerHandler(args interface{}) (IHandler, error) {
	ffprobeInst, err := ffmpeg.NewFFProbe()
	if err != nil {
		logutil.GetLogger(context.Background()).Error("unable to create ffprobe instance, will not able to fix invalid video duration", zap.Error(err))
		ffprobeInst = nil
	}
	return &durationFixer{
		ffprobeInst: ffprobeInst,
	}, nil
}

func init() {
	Register(HDurationFixer, createDurationFixerHandler)
}
