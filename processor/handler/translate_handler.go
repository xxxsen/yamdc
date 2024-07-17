package handler

import (
	"context"
	"fmt"
	"yamdc/model"
	"yamdc/translator"
)

type translaterHandler struct {
}

func (p *translaterHandler) Name() string {
	return HTranslater
}

func (p *translaterHandler) translate(name string, in string, out *string) error {
	if len(in) == 0 {
		return nil
	}
	res, err := translator.Translate(in, "auto", "zh")
	if err != nil {
		return fmt.Errorf("translate failed, name:%s, err:%w", name, err)
	}
	*out = res
	return nil
}

func (p *translaterHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	if !translator.IsTranslatorEnabled() {
		return nil
	}
	var errs []error
	if fc.Meta.ExtInfo.TranslateInfo.Option.EnableTitleTranslate {
		errs = append(errs, p.translate("title", fc.Meta.Title, &fc.Meta.ExtInfo.TranslateInfo.Data.TranslatedTitle))
	}
	if fc.Meta.ExtInfo.TranslateInfo.Option.EnablePlotTranslate {
		errs = append(errs, p.translate("plot", fc.Meta.Plot, &fc.Meta.ExtInfo.TranslateInfo.Data.TranslatedPlot))
	}

	for _, err := range errs {
		if err != nil {
			return fmt.Errorf("translate part failed, err:%w", err)
		}
	}
	return nil
}

func init() {
	Register(HTranslater, HandlerToCreator(&translaterHandler{}))
}
