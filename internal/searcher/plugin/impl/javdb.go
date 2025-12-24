package impl

import (
	"context"
	"fmt"
	"net/http"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/twostep"
)

var defaultJavDBHostList = []string{
	"https://javdb.com",
}

type javdb struct {
	api.DefaultPlugin
}

func (p *javdb) OnGetHosts(ctx context.Context) []string {
	return defaultJavDBHostList
}

func (p *javdb) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	link := fmt.Sprintf("%s/search?q=%s&f=all", api.MustSelectDomain(defaultJavDBHostList), number)
	return http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
}

func (p *javdb) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, req *http.Request) (*http.Response, error) {
	return twostep.HandleXPathTwoStepSearch(ctx, invoker, req, &twostep.XPathTwoStepContext{
		Ps: []*twostep.XPathPair{
			{
				Name:  "read-link",
				XPath: `//div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"]/a/@href`,
			},
			{
				Name:  "read-number",
				XPath: `//div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"]/a/div[@class="video-title"]/strong`,
			},
		},
		LinkSelector: func(ps []*twostep.XPathPair) (string, bool, error) {
			linklist := ps[0].Result
			numberlist := ps[1].Result
			num := number.GetCleanID(meta.GetNumberId(ctx))
			for idx, numberItem := range numberlist {
				link := linklist[idx]
				if number.GetCleanID(numberItem) == num {
					return link, true, nil
				}
			}
			return "", false, nil
		},
		ValidStatusCode:       []int{http.StatusOK},
		CheckResultCountMatch: true,
		LinkPrefix:            fmt.Sprintf("%s://%s", req.URL.Scheme, req.URL.Host),
	})
}

func (p *javdb) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//a[@class="button is-white copy-to-clipboard"]/@data-clipboard-text`,
		TitleExpr:           `//h2[@class="title is-4"]/strong[@class="current-title"]`,
		PlotExpr:            "",
		ActorListExpr:       `//div[strong[contains(text(), "演員")]]/span[@class="value"]/a`,
		ReleaseDateExpr:     `//div[strong[contains(text(), "日期")]]/span[@class="value"]`,
		DurationExpr:        `//div[strong[contains(text(), "時長")]]/span[@class="value"]`,
		StudioExpr:          `//div[strong[contains(text(), "片商")]]/span[@class="value"]`,
		LabelExpr:           "",
		DirectorExpr:        "",
		SeriesExpr:          `//div[strong[contains(text(), "系列")]]/span[@class="value"]`,
		GenreListExpr:       `//div[strong[contains(text(), "類別")]]/span[@class="value"]/a`,
		CoverExpr:           `//div[@class="column column-video-cover"]/a/img/@src`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="tile-images preview-images"]/a[@class="tile-item"]/@href`,
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
	factory.Register(constant.SSJavDB, factory.PluginToCreator(&javdb{}))
}
