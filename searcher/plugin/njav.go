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

type njav struct {
	DefaultPlugin
}

func (p *njav) OnMakeHTTPRequest(ctx *PluginContext, number *number.Number) (*http.Request, error) {
	nid := number.GetNumberID()
	nid = strings.ReplaceAll(nid, "_", "-") //将下划线替换为中划线
	ctx.SetKey("number", nid)
	uri := fmt.Sprintf("https://njavtv.com/cn/search/%s", nid)
	return http.NewRequest(http.MethodGet, uri, nil)
}

func (p *njav) OnHandleHTTPRequest(ctx *PluginContext, invoker HTTPInvoker, req *http.Request) (*http.Response, error) {
	numberId := strings.ToUpper(ctx.GetKeyOrDefault("number", "").(string))
	return HandleXPathTwoStepSearch(ctx, invoker, req, &XPathTwoStepContext{
		Ps: []*XPathPair{
			{
				Name:  "links",
				XPath: `//div[@class="my-2 text-sm text-nord4 truncate"]/a[@class="text-secondary group-hover:text-primary"]/@href`,
			},
			{
				Name:  "title",
				XPath: `//div[@class="my-2 text-sm text-nord4 truncate"]/a[@class="text-secondary group-hover:text-primary"]/text()`,
			},
		},
		LinkSelector: func(ps []*XPathPair) (string, bool, error) {
			links := ps[0].Result
			titles := ps[1].Result
			for i, link := range links {
				title := titles[i]
				title = strings.ToUpper(title)
				if strings.Contains(title, numberId) {
					return link, true, nil
				}
			}
			return "", false, nil
		},
		ValidStatusCode:       []int{http.StatusOK},
		CheckResultCountMatch: true,
	})

}

func (p *njav) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
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
	meta, err := dec.DecodeHTML(data, decoder.WithReleaseDateParser(parser.DefaultReleaseDateParser(ctx.GetContext())))
	if err != nil {
		return nil, false, err
	}
	if len(meta.Number) == 0 {
		return nil, false, nil
	}
	return meta, true, nil
}

func init() {
	Register(SSNJav, PluginToCreator(&njav{}))
}
