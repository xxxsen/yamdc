package impl

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/decoder"
	"yamdc/searcher/parser"
	"yamdc/searcher/plugin/api"
	"yamdc/searcher/plugin/constant"
	"yamdc/searcher/plugin/factory"
	"yamdc/searcher/plugin/meta"
	"yamdc/searcher/plugin/twostep"
)

var (
	defaultMissavDomains = []string{
		"missav.ws",
	}
)

type missav struct {
	api.DefaultPlugin
}

func (p *missav) OnMakeHTTPRequest(ctx context.Context, number *number.Number) (*http.Request, error) {
	link := fmt.Sprintf("https://%s/cn/search/%s", api.MustSelectDomain(defaultMissavDomains), number.GetNumberID())
	return http.NewRequest(http.MethodGet, link, nil)
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

func (p *missav) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.AvMeta, bool, error) {
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
	mdata, err := dec.DecodeHTML(data, decoder.WithReleaseDateParser(parser.DefaultReleaseDateParser(ctx)))
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
