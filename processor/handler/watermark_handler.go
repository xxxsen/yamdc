package handler

import (
	"av-capture/image"
	"av-capture/model"
	"av-capture/store"
	"context"
	"fmt"

	"github.com/xxxsen/common/logutil"
)

type watermark struct {
}

func (h *watermark) Handle(ctx context.Context, fc *model.FileContext) error {
	if fc.Meta.Poster == nil || len(fc.Meta.Poster.Key) == 0 {
		return nil
	}
	data, err := store.GetDefault().GetData(fc.Meta.Poster.Key)
	if err != nil {
		return fmt.Errorf("load poster key failed, key:%s", fc.Meta.Poster.Key)
	}
	tags := make([]image.Watermark, 0, 5)
	if fc.Number.IsUncensorMovie() {
		tags = append(tags, image.WMUncensored)
	}
	if fc.Number.IsChineseSubtitle() {
		tags = append(tags, image.WMChineseSubtitle)
	}
	if len(tags) == 0 {
		logutil.GetLogger(ctx).Debug("no watermark tag found, skip watermark proc")
		return nil
	}
	newData, err := image.AddWatermarkFromBytes(data, tags)
	if err != nil {
		return fmt.Errorf("add watermark failed, err:%w", err)
	}
	key, err := store.GetDefault().Put(newData)
	if err != nil {
		return fmt.Errorf("save watermarked image failed, err:%w", err)
	}
	fc.Meta.Poster.Key = key
	return nil
}

func init() {
	Register(HWatermakrMaker, HandlerToCreator(&watermark{}))
}
