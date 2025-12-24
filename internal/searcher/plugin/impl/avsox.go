package impl

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

const (
	defaultAvsoxSearchExpr = `//*[@id="waterfall"]/div/a/@href`
)

var defaultAvSoxHostList = []string{
	"https://avsox.click",
}

type avsox struct {
	api.DefaultPlugin
}

func (p *avsox) OnGetHosts(ctx context.Context) []string {
	return defaultAvSoxHostList
}

func (p *avsox) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, api.MustSelectDomain(defaultAvSoxHostList), nil) //返回一个假的request
}

func (p *avsox) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, oriReq *http.Request) (*http.Response, error) {
	num := strings.ToUpper(meta.GetNumberId(ctx))
	tryList := p.generateTryList(num)
	logger := logutil.GetLogger(ctx).With(zap.String("plugin", "avsox"))
	logger.Debug("build try list succ", zap.Int("count", len(tryList)), zap.Strings("list", tryList))
	var link string
	var ok bool
	var err error
	for _, item := range tryList {
		link, ok, err = p.trySearchByNumber(ctx, oriReq, invoker, item)
		if err != nil {
			logger.Error("try search number failed", zap.Error(err), zap.String("number", item))
			continue
		}
		if !ok {
			logger.Debug("search item not found, try next", zap.String("number", item))
			continue
		}
		break
	}
	if len(link) == 0 {
		return nil, fmt.Errorf("unable to find match number")
	}
	uri := "https:" + link
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("make request failed, err:%w", err)
	}
	return invoker(ctx, req)
}

func (p *avsox) generateTryList(num string) []string {
	tryList := make([]string, 0, 5)
	tryList = append(tryList, num)
	if strings.Contains(tryList[len(tryList)-1], "-") {
		tryList = append(tryList, strings.ReplaceAll(tryList[len(tryList)-1], "-", "_"))
	}
	if strings.Contains(tryList[len(tryList)-1], "_") {
		tryList = append(tryList, strings.ReplaceAll(tryList[len(tryList)-1], "_", ""))
	}
	return tryList
}

func (p *avsox) trySearchByNumber(ctx context.Context, oriReq *http.Request, invoker api.HTTPInvoker, number string) (string, bool, error) {
	host := fmt.Sprintf("%s://%s", oriReq.URL.Scheme, oriReq.URL.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/cn/search/%s", host, number), nil)
	if err != nil {
		return "", false, err
	}
	rsp, err := invoker(ctx, req)
	if err != nil {
		return "", false, err
	}
	defer rsp.Body.Close()
	tree, err := utils.ReadDataAsHTMLTree(rsp)
	if err != nil {
		return "", false, err
	}
	tmp := decoder.DecodeList(tree, defaultAvsoxSearchExpr)
	res := make([]string, 0, len(tmp))
	for _, item := range tmp {
		if strings.Contains(item, "movie") {
			res = append(res, item)
		}
	}
	if len(res) == 0 {
		return "", false, fmt.Errorf("no search item found")
	}
	if len(res) > 1 {
		return "", false, fmt.Errorf("too much search item, cnt:%d", len(res))
	}
	return res[0], true, nil
}

func (p *avsox) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//span[contains(text(),"识别码:")]/../span[2]/text()`,
		TitleExpr:           `/html/body/div[2]/h3/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//a[@class="avatar-box"]/span/text()`,
		ReleaseDateExpr:     `//span[contains(text(),"发行时间:")]/../text()`,
		DurationExpr:        `//p[span[contains(text(), "长度")]]/text()`,
		StudioExpr:          `//p[contains(text(),"制作商: ")]/following-sibling::p[1]/a/text()`,
		LabelExpr:           ``,
		DirectorExpr:        "",
		SeriesExpr:          `//p[contains(text(),"系列:")]/following-sibling::p[1]/a/text()`,
		GenreListExpr:       `//p[span[@class="genre"]]/span/a[contains(@href, "genre")]`,
		CoverExpr:           `/html/body/div[2]/div[1]/div[1]/a/img/@src`,
		PosterExpr:          "",
		SampleImageListExpr: "",
	}
	meta, err := dec.DecodeHTML(data,
		decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)),
		decoder.WithDurationParser(parser.DefaultDurationParser(ctx)),
		decoder.WithDefaultStringProcessor(strings.TrimSpace),
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
	factory.Register(constant.SSAvsox, factory.PluginToCreator(&avsox{}))
}
