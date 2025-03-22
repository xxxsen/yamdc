package handler

import (
	"context"
	"fmt"
	"time"
	"yamdc/enum"
	"yamdc/hasher"
	"yamdc/model"
	"yamdc/store"
	"yamdc/translator"
)

const (
	defaultTranslateDataSaveTime = 30 * 24 * time.Hour
)

type translaterHandler struct {
}

func (p *translaterHandler) Name() string {
	return HTranslater
}

func (p *translaterHandler) buildKey(data string) string {
	return fmt.Sprintf("yamdc:translate:%s", hasher.ToMD5(data))
}

func (p *translaterHandler) translateSingle(ctx context.Context, name string, in string, lang string, out *string) error {
	if len(in) == 0 {
		return nil
	}
	if len(lang) == 0 || lang == enum.MetaLangZHTW || lang == enum.MetaLangZH {
		return nil
	}
	res, err := store.LoadData(ctx, p.buildKey(in), defaultTranslateDataSaveTime, func() ([]byte, error) {
		res, err := translator.Translate(ctx, in, "auto", "zh")
		if err != nil {
			return nil, err
		}
		return []byte(res), nil
	})

	if err != nil {
		return fmt.Errorf("translate failed, name:%s, data:%s, err:%w", name, in, err)
	}
	*out = string(res)
	return nil
}

func (p *translaterHandler) translateArray(ctx context.Context, name string, in []string, lang string, out *[]string) error {
	rs := make([]string, 0, len(in))
	for _, item := range in {
		var res string
		if err := p.translateSingle(ctx, "dispatch-"+name+"-translate", item, lang, &res); err != nil {
			return err
		}
		rs = append(rs, fmt.Sprintf("%s/%s", res, item))
	}
	*out = rs
	return nil
}

func (p *translaterHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	if !translator.IsTranslatorEnabled() {
		return nil
	}
	var errs []error
	errs = append(errs, p.translateSingle(ctx, "title", fc.Meta.Title, fc.Meta.TitleLang, &fc.Meta.TitleTranslated))
	errs = append(errs, p.translateSingle(ctx, "plot", fc.Meta.Plot, fc.Meta.PlotLang, &fc.Meta.PlotTranslated))
	errs = append(errs, p.translateArray(ctx, "genere", fc.Meta.Genres, fc.Meta.GenresLang, &fc.Meta.Genres))
	errs = append(errs, p.translateArray(ctx, "actor", fc.Meta.Actors, fc.Meta.ActorsLang, &fc.Meta.Actors))

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
