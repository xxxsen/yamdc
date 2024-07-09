package plugin

import (
	"context"
	"fmt"
	"net/http"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/decoder"
	"yamdc/searcher/utils"
)

type javhoo struct {
	DefaultPlugin
}

func (p *javhoo) OnMakeHTTPRequest(ctx *PluginContext, number *number.Number) (*http.Request, error) {
	uri := fmt.Sprintf("https://www.javhoo.com/av/%s", number.Number())
	return http.NewRequest(http.MethodGet, uri, nil)
}

func (p *javhoo) onParseDuration(_ context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		rs, _ := utils.ToDuration(v)
		return rs
	}
}

func (p *javhoo) onParseReleaseDate(_ context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		rs, _ := utils.ToTimestamp(v)
		return rs
	}
}

func (p *javhoo) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//div[@class="project_info"]/p/span[@class="categories"]/text()`,
		TitleExpr:           `//header[@class="article-header"]/h1[@class="article-title"]/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//p/span[@class="genre"]/a[contains(@href, "star")]/text()`,
		ReleaseDateExpr:     `//div[@class="project_info"]/p[span[contains(text(), "發行日期")]]/text()[2]`,
		DurationExpr:        `//div[@class="project_info"]/p[span[contains(text(), "長度")]]/text()[2]`,
		StudioExpr:          `//div[@class="project_info"]/p[span[contains(text(), "製作商")]]/a/text()`,
		LabelExpr:           `//div[@class="project_info"]/p[span[contains(text(), "發行商")]]/a/text()`,
		DirectorExpr:        `//div[@class="project_info"]/p[span[contains(text(), "導演")]]/a/text()`,
		SeriesExpr:          `//div[@class="project_info"]/p[span[contains(text(), "系列")]]/a/text()`,
		GenreListExpr:       `//p/span[@class="genre"]/a[contains(@href, "genre")]/text()`,
		CoverExpr:           `//p/a[@class="dt-single-image"]/@href`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@id="sample-box"]/div/a/@href`,
	}
	meta, err := dec.DecodeHTML(data,
		decoder.WithReleaseDateParser(p.onParseReleaseDate(ctx.GetContext())),
		decoder.WithDurationParser(p.onParseDuration(ctx.GetContext())),
	)
	if err != nil {
		return nil, false, err
	}
	if len(meta.Number) == 0 {
		return nil, false, nil
	}
	return meta, true, nil
}

func init() {
	Register(SSJavhoo, PluginToCreator(&javhoo{}))
}
