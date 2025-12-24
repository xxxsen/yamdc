package impl

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

var defaultNJavHostList = []string{
	"https://njavtv.com",
}

type njav struct {
	api.DefaultPlugin
}

func (p *njav) OnGetHosts(ctx context.Context) []string {
	return defaultNJavHostList
}

func (p *njav) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	nid := number
	nid = strings.ReplaceAll(nid, "_", "-") //将下划线替换为中划线
	uri := fmt.Sprintf("%s/cn/search/%s", api.MustSelectDomain(defaultNJavHostList), nid)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (p *njav) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, req *http.Request) (*http.Response, error) {
	cleanNumberId := strings.ToUpper(number.GetCleanID(meta.GetNumberId(ctx)))
	return twostep.HandleXPathTwoStepSearch(ctx, invoker, req, &twostep.XPathTwoStepContext{
		Ps: []*twostep.XPathPair{
			{
				Name:  "links",
				XPath: `//div[@class="my-2 text-sm text-nord4 truncate"]/a[@class="text-secondary group-hover:text-primary"]/@href`,
			},
			{
				Name:  "title",
				XPath: `//div[@class="my-2 text-sm text-nord4 truncate"]/a[@class="text-secondary group-hover:text-primary"]/text()`,
			},
		},
		LinkSelector: func(ps []*twostep.XPathPair) (string, bool, error) {
			links := ps[0].Result
			titles := ps[1].Result
			for i, link := range links {
				title := titles[i]
				title = strings.ToUpper(number.GetCleanID(title))
				if strings.Contains(title, cleanNumberId) {
					return link, true, nil
				}
			}
			return "", false, nil
		},
		ValidStatusCode:       []int{http.StatusOK},
		CheckResultCountMatch: true,
	})

}

func (p *njav) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//div[@class="text-secondary" and contains(span[text()], "番号:")]/span[@class="font-medium"]/text()`,
		TitleExpr:           `//div[@class="text-secondary" and contains(span[text()], "标题:")]/span[@class="font-medium"]/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//meta[@property="og:video:actor"]/@content`,
		ReleaseDateExpr:     `//div[@class="text-secondary" and contains(span[text()], "发行日期:")]/time[@class="font-medium"]/text()`,
		DurationExpr:        `//meta[@property="og:video:duration"]/@content`,
		StudioExpr:          `//div[@class="text-secondary" and contains(span[text()], "发行商:")]/a[@class="text-nord13 font-medium"]/text()`,
		LabelExpr:           `//div[@class="text-secondary" and contains(span[text()], "标籤:")]/a[@class="text-nord13 font-medium"]/text()`,
		DirectorExpr:        `//div[@class="text-secondary" and contains(span[text()], "导演:")]/a[@class="text-nord13 font-medium"]/text()`,
		SeriesExpr:          `//div[@class="text-secondary" and contains(span[text()], "系列:")]/a[@class="text-nord13 font-medium"]/text()`,
		GenreListExpr:       `//div[@class="text-secondary" and contains(span[text()], "类型:")]/a[@class="text-nord13 font-medium"]/text()`,
		CoverExpr:           `//link[@rel="preload" and @as="image"]/@href`,
		PosterExpr:          "",
		SampleImageListExpr: "",
	}
	meta, err := dec.DecodeHTML(data, decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)))
	if err != nil {
		return nil, false, err
	}
	if len(meta.Number) == 0 {
		return nil, false, nil
	}
	return meta, true, nil
}

func init() {
	factory.Register(constant.SSNJav, factory.PluginToCreator(&njav{}))
}
