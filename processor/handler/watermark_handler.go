package handler

import (
	"av-capture/model"
	"context"
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
	//TODO: 尝试添加水印
	return nil
}
