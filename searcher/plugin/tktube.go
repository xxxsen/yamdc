package plugin

import (
	"fmt"
	"net/http"
	"strings"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/decoder"
	"yamdc/searcher/parser"
)

type tktube struct {
	DefaultPlugin
}

func (p *tktube) OnMakeHTTPRequest(ctx *PluginContext, n *number.Number) (*http.Request, error) {
	nid := strings.ReplaceAll(n.GetNumberID(), "-", "--")
	uri := fmt.Sprintf("https://tktube.com/zh/search/%s/", nid)
	return http.NewRequest(http.MethodGet, uri, nil)
}

func (p *tktube) OnHandleHTTPRequest(ctx *PluginContext, invoker HTTPInvoker, req *http.Request) (*http.Response, error) {
	numberId := strings.ToUpper(ctx.MustGetNumberInfo().GetNumberID())
	return HandleXPathTwoStepSearch(ctx, invoker, req, &XPathTwoStepContext{
		Ps: []*XPathPair{
			{
				Name:  "links",
				XPath: `//div[@id="list_videos_videos_list_search_result_items"]/div/a/@href`,
			},
			{
				Name:  "names",
				XPath: `//div[@id="list_videos_videos_list_search_result_items"]/div/a/strong[@class="title"]/text()`,
			},
		},
		LinkSelector: func(ps []*XPathPair) (string, bool, error) {
			links := ps[0].Result
			names := ps[1].Result
			for i := 0; i < len(links); i++ {
				if strings.Contains(strings.ToUpper(names[i]), numberId) {
					return links[i], true, nil
				}
			}
			return "", false, nil
		},
		ValidStatusCode:       []int{http.StatusOK},
		CheckResultCountMatch: true,
		LinkPrefix:            "",
	})
}

func (p *tktube) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		TitleExpr:           `//div[@class="headline"]/h1/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//div[contains(text(), "女優:")]/a[contains(@href, "models")]/text()`,
		ReleaseDateExpr:     `//div[@class="item"]/span[contains(text(), "加入日期:")]/em/text()`,
		DurationExpr:        `//div[@class="item"]/span[contains(text(), "時長:")]/em/text()`,
		StudioExpr:          "",
		LabelExpr:           "",
		DirectorExpr:        "",
		SeriesExpr:          "",
		GenreListExpr:       `//div[contains(text(), "標籤:")]/a[contains(@href, "tags")]/text()`,
		CoverExpr:           `//meta[@property="og:image"]/@content`,
		PosterExpr:          "",
		SampleImageListExpr: "",
	}
	meta, err := dec.DecodeHTML(data,
		decoder.WithDurationParser(parser.DefaultHHMMSSDurationParser(ctx.GetContext())),
		decoder.WithReleaseDateParser(parser.DefaultReleaseDateParser(ctx.GetContext())),
	)
	if err != nil {
		return nil, false, err
	}
	meta.Number = ctx.MustGetNumberInfo().GetNumberID()
	return meta, true, nil
}

func init() {
	Register(SSTKTube, PluginToCreator(&tktube{}))
}
