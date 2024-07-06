package handler

import (
	"av-capture/image"
	"av-capture/model"
	"av-capture/store"
	"context"
	"fmt"
)

type watermark struct {
}

func (h *watermark) Handle(ctx context.Context, fc *model.FileContext) error {
	if !fc.Number.IsChineseSubtitle() {
		return nil
	}
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
