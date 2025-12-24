package impl

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/twostep"
)

var (
	defaultMissavDomains = []string{
		"https://missav.ws",
	}
)

type missav struct {
	api.DefaultPlugin
}

func (p *missav) OnGetHosts(ctx context.Context) []string {
	return defaultMissavDomains
}

func (p *missav) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	link := fmt.Sprintf("%s/cn/search/%s", api.MustSelectDomain(defaultMissavDomains), number)
	return http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
}

func (p *missav) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, req *http.Request) (*http.Response, error) {
	xctx := &twostep.XPathTwoStepContext{
		Ps: []*twostep.XPathPair{
			{
				Name:  "read-link",
				XPath: `//div[@class="my-2 text-sm text-nord4 truncate"]/a[@class="text-secondary group-hover:text-primary"]/@href`,
			},
			{
				Name:  "read-title",
				XPath: `//div[@class="my-2 text-sm text-nord4 truncate"]/a[@class="text-secondary group-hover:text-primary"]/text()`,
			},
		},
		LinkSelector: func(ps []*twostep.XPathPair) (string, bool, error) {
			linkList := ps[0].Result
			titleList := ps[1].Result
			for i, link := range linkList {
				title := titleList[i]
				if strings.Contains(title, meta.GetNumberId(ctx)) {
					return link, true, nil
				}
			}
			return "", false, nil
		},
		ValidStatusCode:       []int{http.StatusOK},
		CheckResultCountMatch: true,
		LinkPrefix:            "",
	}
	return twostep.HandleXPathTwoStepSearch(ctx, invoker, req, xctx)
}

func (p *missav) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//div[span[contains(text(), "番号")]]/span[@class="font-medium"]/text()`,
		TitleExpr:           `//div[@class="mt-4"]/h1[@class="text-base lg:text-lg text-nord6"]/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//div[span[contains(text(), "女优")]]/a/text()`,
		ReleaseDateExpr:     `//div[span[contains(text(), "发行日期")]]/time/text()`,
		DurationExpr:        "",
		StudioExpr:          `//div[span[contains(text(), "发行商")]]/a/text()`,
		LabelExpr:           "",
		DirectorExpr:        `//div[span[contains(text(), "导演")]]/a/text()`,
		SeriesExpr:          "",
		GenreListExpr:       `//div[span[contains(text(), "类型")]]/a/text()`,
		CoverExpr:           `//link[@rel="preload" and @as="image"]/@href`,
		PosterExpr:          "",
		SampleImageListExpr: "",
	}
	mdata, err := dec.DecodeHTML(data, decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)))
	if err != nil {
		return nil, false, err
	}
	if len(mdata.Number) == 0 {
		return nil, false, err
	}
	return mdata, true, nil
}

func init() {
	factory.Register(constant.SSMissav, factory.PluginToCreator(&missav{}))
}
