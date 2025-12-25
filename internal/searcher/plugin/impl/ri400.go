package impl

import (
	"context"
	"fmt"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/twostep"
	"net/http"
	"strings"
)

var (
	defaultRi400HostList = []string{
		"https://ri400.xyz",
	}
)

type ri400 struct {
	api.DefaultPlugin
}

func (m *ri400) cleanNumber(num string) string {
	num = strings.TrimPrefix(num, "MADOU") //移除默认的前缀
	num = strings.Trim(num, "-_")
	return num
}

func (m *ri400) OnGetHosts(ctx context.Context) []string {
	return defaultRi400HostList
}

func (m *ri400) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	if !strings.HasPrefix(number, "MADOU") {
		return false, nil
	}
	return true, nil
}

func (m *ri400) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	num := m.cleanNumber(number)
	link := fmt.Sprintf("%s/search?content=%s", api.MustSelectDomain(defaultRi400HostList), num)
	return http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
}

func (m *ri400) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, req *http.Request) (*http.Response, error) {
	return twostep.HandleXPathTwoStepSearch(ctx, invoker, req, &twostep.XPathTwoStepContext{
		Ps: []*twostep.XPathPair{
			{
				Name:  "read-link",
				XPath: `//div[@class="main"]/div/ul[@class="row"]/li/div[@class="img-item mb5"]/a[contains(@href,'/category') and not (contains(@class, 'img-ico'))]/@href`,
			},
			{
				Name:  "read-number",
				XPath: `//div[@class="main"]//div//ul[@class="row"]/li/p/span`,
			},
		},
		LinkSelector: func(ps []*twostep.XPathPair) (string, bool, error) {
			linkList := ps[0].Result
			numberList := ps[1].Result

			num := m.cleanNumber(meta.GetNumberId(ctx))

			for idx, numberAndTitleItem := range numberList {
				link := linkList[idx]
				if strings.Contains(numberAndTitleItem, num) {
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

func (m *ri400) onDecodeNumber(in string) string {
	// "[XWJ-0001]xxx标题"
	// "纯标题"

	start := strings.Index(in, "[")
	end := strings.Index(in, "]")

	if start == -1 || end == -1 || end <= start {
		return ""
	}
	number := in[start+1 : end]
	number = strings.TrimSpace(number)
	number = strings.ToUpper(number)
	return number
}

func (m *ri400) onDecodeTitle(in string) string {
	// "[XWJ-0001]xxx标题"
	// "纯标题"
	idx := strings.Index(in, "]")
	if idx == -1 {
		return strings.TrimSpace(in)
	}

	title := in[idx+1:]
	title = strings.TrimSpace(title)
	return title
}

func (m *ri400) onDecodeDuration(in string) int64 {
	// xx分yy秒,目前没有遇到超过1小时的
	in = strings.ReplaceAll(in, "小时", "h")
	in = strings.ReplaceAll(in, "分", "m")
	in = strings.ReplaceAll(in, "秒", "s")
	return parser.HumanDurationToSecond(in)
}

func (m *ri400) onDecodeGenres(in []string) []string {
	for i, tag := range in {
		in[i] = strings.ReplaceAll(tag, "?", "")
	}
	return in
}

func (m *ri400) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//div[@id='download']/div[1]//h3/text()`,
		TitleExpr:           `//div[@id='download']/div[1]//h3/text()`,
		PlotExpr:            `//div[@id='download']/div[2]//ul[contains(@class,'info-txt')]/p/text()`,
		ActorListExpr:       `//div[@id='download']/div[2]//a[contains(@class,'list-click')]/h3/text()`,
		ReleaseDateExpr:     ``,
		DurationExpr:        `//div[@id='download']/div[1]/div/p/span[2]/text()`,
		StudioExpr:          ``,
		LabelExpr:           ``,
		DirectorExpr:        ``,
		SeriesExpr:          ``,
		GenreListExpr:       `//div[@id='download']/div[2]//div[contains(@class,'info-tag')]/a/text()`,
		CoverExpr:           `//div[@id='download']/div[1]//span[@class='img-box']/video/@poster`,
		PosterExpr:          `//div[@id='download']/div[1]//span[@class='img-box']/video/@poster`,
		SampleImageListExpr: "//dd[@class='thisclass']//p/img/@data-src",
	}
	mv, err := dec.DecodeHTML(data,
		decoder.WithNumberParser(m.onDecodeNumber),
		decoder.WithTitleParser(m.onDecodeTitle),
		decoder.WithDurationParser(m.onDecodeDuration),
		decoder.WithGenreListParser(m.onDecodeGenres),
	)
	if err != nil {
		return nil, false, err
	}
	if len(mv.Number) == 0 {
		mv.Number = meta.GetNumberId(ctx)
	}
	mv.TitleLang = enum.MetaLangZHTW
	mv.PlotLang = enum.MetaLangZHTW
	mv.SwithConfig.DisableReleaseDateCheck = true
	return mv, true, nil
}

func init() {
	factory.Register(constant.SSRi400, factory.PluginToCreator(&ri400{}))
}
