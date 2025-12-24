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

var default18AvHostList = []string{
	"https://18av.me",
}

type av18 struct {
	api.DefaultPlugin
}

func (p *av18) OnGetHosts(ctx context.Context) []string {
	return default18AvHostList
}

func (p *av18) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	host := api.MustSelectDomain(default18AvHostList)
	uri := fmt.Sprintf("%s/cn/search.php?kw_type=key&kw=%s", host, number)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (p *av18) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, req *http.Request) (*http.Response, error) {
	xctx := &twostep.XPathTwoStepContext{
		Ps: []*twostep.XPathPair{
			{
				Name:  "read-link",
				XPath: `//div[@class="content flex-columns small px-2"]/span[@class="title"]/a/@href`,
			},
			{
				Name:  "read-title",
				XPath: `//div[@class="content flex-columns small px-2"]/span[@class="title"]/a/text()`,
			},
		},
		LinkSelector: func(ps []*twostep.XPathPair) (string, bool, error) {
			number := strings.ToUpper(meta.GetNumberId(ctx))
			linkList := ps[0].Result
			titleList := ps[1].Result
			for idx, link := range linkList {
				title := titleList[idx]
				if strings.Contains(strings.ToUpper(title), number) {
					return link, true, nil
				}
			}
			return "", false, nil
		},
		ValidStatusCode:       []int{http.StatusOK},
		CheckResultCountMatch: true,
		LinkPrefix:            fmt.Sprintf("%s://%s/cn", req.URL.Scheme, req.URL.Host),
	}
	return twostep.HandleXPathTwoStepSearch(ctx, invoker, req, xctx)
}

func (p *av18) coverParser(in string) string {
	return strings.ReplaceAll(in, " ", "")
}

func (p *av18) plotParser(in string) string {
	return strings.TrimSpace(strings.TrimLeft(in, "简介："))
}

func (p *av18) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//div[@class="px-0 flex-columns"]/div[@class="number"]/text()`,
		TitleExpr:           `//div[@class="d-flex px-3 py-2 name col bg-w"]/h1[@class="h4 b"]/text()`,
		PlotExpr:            `//div[@class="intro  bd-light w-100 mt-1"]/p[contains(text(), '简介：')]/text()`,
		ActorListExpr:       `//div[@class="d-flex col px-0 tag-info flex-wrap mt-2 pt-2 bd-top bd-primary"]/a/span[@itemprop="name"]/text()`,
		ReleaseDateExpr:     `//div[@class="date"]/text()`,
		DurationExpr:        "",
		StudioExpr:          "",
		LabelExpr:           "",
		DirectorExpr:        "",
		SeriesExpr:          `//div[@class="bd-top my-1 align-items-center"]/a[@class="btn btn-ripple border-pill px-3 mr-2 my-1 bg-primary"]`,
		GenreListExpr:       `//div[@class="d-flex col px-0 tag-info flex-wrap mt-2 pt-2 bd-top bd-primary"]/a[contains(@href, "s_type=tag")]/text()`,
		CoverExpr:           `//meta[@property="og:image"]/@content`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="cover"]/a/img/@data-src`,
	}
	meta, err := dec.DecodeHTML(data,
		decoder.WithCoverParser(p.coverParser),
		decoder.WithPlotParser(p.plotParser),
		decoder.WithDurationParser(parser.DefaultDurationParser(ctx)),
		decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)),
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
	factory.Register(constant.SS18AV, factory.PluginToCreator(&av18{}))
}
