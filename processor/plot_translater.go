package processor

import (
	"av-capture/model"
	"av-capture/translater"
	"context"
	"fmt"
)

type translaterProcessor struct {
}

func (p *translaterProcessor) Name() string {
	return PsTranslater
}

func (p *translaterProcessor) Process(ctx context.Context, fc *model.FileContext) error {
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

func (p *translaterProcessor) IsOptional() bool {
	return true
}
