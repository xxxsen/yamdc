package impl

import (
	"context"
	"net/http"
	"yamdc/enum"
	"yamdc/model"
	"yamdc/searcher/decoder"
	"yamdc/searcher/parser"
	"yamdc/searcher/plugin/api"
	"yamdc/searcher/plugin/constant"
	"yamdc/searcher/plugin/factory"
	"yamdc/searcher/plugin/meta"
)

var defaultFreeJavBtHostList = []string{
	"https://freejavbt.com",
}

type freejavbt struct {
	api.DefaultPlugin
}

func (p *freejavbt) OnGetHosts(ctx context.Context) []string {
	return defaultFreeJavBtHostList
}

func (p *freejavbt) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	host := api.MustSelectDomain(defaultFreeJavBtHostList)
	uri := host + "/zh/" + number
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (p *freejavbt) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          "",
		TitleExpr:           `//h1[@class="text-white"]/strong/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//div[span[contains(text(), "女优")]]/div/a/text()`,
		ReleaseDateExpr:     `//div[span[contains(text(), "日期")]]/span[2]`,
		DurationExpr:        `//div[span[contains(text(), "时长")]]/span[2]`,
		StudioExpr:          `//div[span[contains(text(), "制作")]]/a`,
		LabelExpr:           "",
		DirectorExpr:        `//div[span[contains(text(), "导演")]]/a`,
		SeriesExpr:          "",
		GenreListExpr:       `//div[span[contains(text(), "类别")]]/div/a/text()`,
		CoverExpr:           `//img[@class="video-cover rounded lazyload"]/@data-src`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="preview"]/a/img/@data-src`,
	}
	res, err := dec.DecodeHTML(data,
		decoder.WithDurationParser(parser.DefaultDurationParser(ctx)),
		decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	res.Number = meta.GetNumberId(ctx)
	res.TitleLang = enum.MetaLangJa
	return res, true, nil
}

func init() {
	factory.Register(constant.SSFreeJavBt, factory.PluginToCreator(&freejavbt{}))
}
