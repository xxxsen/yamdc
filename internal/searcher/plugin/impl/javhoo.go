package impl

import (
	"context"
	"fmt"
	"net/http"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
)

var defaultJavHooHostList = []string{
	"https://www.javhoo.com",
}

type javhoo struct {
	api.DefaultPlugin
}

func (p *javhoo) OnGetHosts(ctx context.Context) []string {
	return defaultJavHooHostList
}

func (p *javhoo) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	uri := fmt.Sprintf("%s/av/%s", api.MustSelectDomain(defaultJavHooHostList), number)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (p *javhoo) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
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
		decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)),
		decoder.WithDurationParser(parser.DefaultDurationParser(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	if len(meta.Number) == 0 {
		return nil, false, nil
	}
	meta.TitleLang = enum.MetaLangJa
	return meta, true, nil
}

func init() {
	factory.Register(constant.SSJavhoo, factory.PluginToCreator(&javhoo{}))
}
