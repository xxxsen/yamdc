package handler

import (
	"context"
	"strings"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
)

type numberTitleHandler struct {
}

func (h *numberTitleHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	title := number.GetCleanID(fc.Meta.Title)
	num := number.GetCleanID(fc.Number.GetNumberID())
	if strings.Contains(title, num) {
		return nil
	}
	fc.Meta.Title = fc.Number.GetNumberID() + " " + fc.Meta.Title
	return nil
}

func init() {
	Register(HNumberTitle, HandlerToCreator(&numberTitleHandler{}))
}
