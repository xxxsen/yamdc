package impl

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
)

var defaultFreeJav321HostList = []string{
	"https://www.jav321.com",
}

type jav321 struct {
	api.DefaultPlugin
}

func (p *jav321) OnGetHosts(ctx context.Context) []string {
	return defaultFreeJav321HostList
}

func (p *jav321) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	data := url.Values{}
	data.Set("sn", number)
	body := data.Encode()
	host := api.MustSelectDomain(defaultFreeJav321HostList)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/search", host), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(body)))
	return req, nil
}

func (s *jav321) defaultStringProcessor(v string) string {
	v = strings.Trim(v, ": \t")
	return strings.TrimSpace(v)
}

func (p *jav321) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := &decoder.XPathHtmlDecoder{
		NumberExpr:          `//b[contains(text(),"品番")]/following-sibling::node()`,
		TitleExpr:           `/html/body/div[2]/div[1]/div[1]/div[1]/h3/text()`,
		PlotExpr:            `/html/body/div[2]/div[1]/div[1]/div[2]/div[3]/div/text()`,
		ActorListExpr:       `//b[contains(text(),"出演者")]/following-sibling::a[starts-with(@href,"/star")]/text()`,
		ReleaseDateExpr:     `//b[contains(text(),"配信開始日")]/following-sibling::node()`,
		DurationExpr:        `//b[contains(text(),"収録時間")]/following-sibling::node()`,
		StudioExpr:          `//b[contains(text(),"メーカー")]/following-sibling::a[starts-with(@href,"/company")]/text()`,
		LabelExpr:           `//b[contains(text(),"メーカー")]/following-sibling::a[starts-with(@href,"/company")]/text()`,
		SeriesExpr:          `//b[contains(text(),"シリーズ")]/following-sibling::node()`,
		GenreListExpr:       `//b[contains(text(),"ジャンル")]/following-sibling::a[starts-with(@href,"/genre")]/text()`,
		CoverExpr:           `/html/body/div[2]/div[2]/div[1]/p/a/img/@src`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="col-md-3"]/div[@class="col-xs-12 col-md-12"]/p/a/img/@src`,
	}
	rs, err := dec.DecodeHTML(data,
		decoder.WithDefaultStringProcessor(p.defaultStringProcessor),
		decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)),
		decoder.WithDurationParser(parser.DefaultDurationParser(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	rs.TitleLang = enum.MetaLangJa
	rs.PlotLang = enum.MetaLangJa
	return rs, true, nil
}

func init() {
	factory.Register(constant.SSJav321, factory.PluginToCreator(&jav321{}))
}
