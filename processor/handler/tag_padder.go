package handler

import (
	"context"
	"yamdc/model"
	"yamdc/utils"
)

type tagPadder struct{}

func (h *tagPadder) Handle(ctx context.Context, fc *model.FileContext) error {
	fc.Meta.Genres = append(fc.Meta.Genres, fc.Number.GenerateTags()...)
	fc.Meta.Genres = utils.DedupStringList(fc.Meta.Genres)
	return nil
}

func init() {
	Register(HTagPadder, HandlerToCreator(&tagPadder{}))
}
