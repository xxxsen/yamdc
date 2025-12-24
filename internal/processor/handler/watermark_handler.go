package handler

import (
	"context"
	"fmt"
	"github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"

	"github.com/xxxsen/common/logutil"
)

type watermark struct {
}

func (h *watermark) Handle(ctx context.Context, fc *model.FileContext) error {
	if fc.Meta.Poster == nil || len(fc.Meta.Poster.Key) == 0 {
		return nil
	}
	tags := make([]image.Watermark, 0, 5)
	if fc.Number.GetIs4K() {
		tags = append(tags, image.WM4K)
	}
	if fc.Number.GetIs8K() {
		tags = append(tags, image.WM8K)
	}
	if fc.Number.GetIsVR() {
		tags = append(tags, image.WMVR)
	}
	if fc.Number.GetExternalFieldUncensor() {
		tags = append(tags, image.WMUncensored)
	}
	if fc.Number.GetIsChineseSubtitle() {
		tags = append(tags, image.WMChineseSubtitle)
	}
	if fc.Number.GetIsLeak() {
		tags = append(tags, image.WMLeak)
	}
	if fc.Number.GetIsHack() {
		tags = append(tags, image.WMHack)
	}
	if len(tags) == 0 {
		logutil.GetLogger(ctx).Debug("no watermark tag found, skip watermark proc")
		return nil
	}
	key, err := store.AnonymousDataRewrite(ctx, fc.Meta.Poster.Key, func(ctx context.Context, data []byte) ([]byte, error) {
		return image.AddWatermarkFromBytes(data, tags)
	})
	if err != nil {
		return fmt.Errorf("save watermarked image failed, err:%w", err)
	}
	fc.Meta.Poster.Key = key
	return nil
}

func init() {
	Register(HWatermakrMaker, HandlerToCreator(&watermark{}))
}
