package handler

import (
	"context"
	"yamdc/constant"
	"yamdc/model"
	"yamdc/utils"
)

type tagPadder struct{}

func (h *tagPadder) Handle(ctx context.Context, fc *model.FileContext) error {
	if fc.Number.GetIsUncensorMovie() {
		fc.Meta.Genres = append(fc.Meta.Genres, constant.TagUncensored)
	}
	if fc.Number.GetIsChineseSubtitle() {
		fc.Meta.Genres = append(fc.Meta.Genres, constant.TagChineseSubtitle)
	}
	if fc.Number.GetIs4K() {
		fc.Meta.Genres = append(fc.Meta.Genres, constant.Tag4K)
	}
	if fc.Number.GetIsLeak() {
		fc.Meta.Genres = append(fc.Meta.Genres, constant.TagLeak)
	}
	fc.Meta.Genres = utils.DedupStringList(fc.Meta.Genres)
	return nil
}

func init() {
	Register(HTagPadder, HandlerToCreator(&tagPadder{}))
}
