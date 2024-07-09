package handler

import (
	"context"
	"fmt"
	"yamdc/model"
	"yamdc/translater"
)

type plotTranslaterHandler struct {
}

func (p *plotTranslaterHandler) Name() string {
	return HPlotTranslater
}

func (p *plotTranslaterHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	if len(fc.Meta.Plot) == 0 {
		return nil
	}
	res, err := translater.GetDefault().Translate(fc.Meta.Plot, "auto", "zh")
	if err != nil {
		return fmt.Errorf("translate plot failed, err:%w", err)
	}
	fc.Meta.ExtInfo.TranslatedPlot = res
	return nil
}

func init() {
	Register(HPlotTranslater, HandlerToCreator(&plotTranslaterHandler{}))
}
