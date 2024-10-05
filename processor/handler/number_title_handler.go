package handler

import (
	"context"
	"strings"
	"yamdc/model"
)

type numberTitleHandler struct {
}

func (c *numberTitleHandler) cleanText(in string) string {
	in = strings.ReplaceAll(in, "_", "")
	in = strings.ReplaceAll(in, "-", "")
	in = strings.ReplaceAll(in, " ", "")
	in = strings.ToUpper(in)
	return in
}

func (c *numberTitleHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	nid := c.cleanText(fc.Number.GetNumberID())
	title := c.cleanText(fc.Meta.Title)
	if strings.Contains(title, nid) {
		return nil
	}
	fc.Meta.Title = fc.Number.GetNumberID() + " " + fc.Meta.Title
	return nil
}

func init() {
	Register(HNumberTitle, HandlerToCreator(&numberTitleHandler{}))
}
