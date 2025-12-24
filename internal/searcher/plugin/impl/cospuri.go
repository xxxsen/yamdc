package impl

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/twostep"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

var defaultCosPuriHosts = []string{
	"https://www.cospuri.com",
}

var (
	defaultCosPuriV1NumberFormatRegexp = regexp.MustCompile(`^[0-9]{4}([a-zA-Z0-9]{0,4})?$`)
	defaultCosPuriV2NumberFormatRegexp = regexp.MustCompile(`^[0-9]{4}[a-zA-Z0-9]{4}$`)
	defaultCosPuriCoverMatchRegexp     = regexp.MustCompile(`(?i)url\((.*)\s*\)`)
)

const (
	defaultCospuriRealNumberIdKey = "key_cospuri_real_number_id"
)

type cospuri struct {
	api.DefaultPlugin
}

func (c *cospuri) OnGetHosts(ctx context.Context) []string {
	return defaultCosPuriHosts
}

func (c *cospuri) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	//2种番号格式
	//格式v1: COSPURI-XXXX[-YYYY]:n-{NUMBER:4}[{ALNUM:4}]
	// example: COSPURI-Emiri-Momota-0548cpar, 最后4个字符可以忽略 => COSPURI-Emiri-Momota-0548
	// 前面的前后PART是固定的, 中间任意长度, COSPURI 只是前缀, 只用于识别, 没有任何意义
	//格式v2: COSPURI-ID, ID由数字字母组成, 可以拼接获取到完整的链接
	if strings.HasPrefix(number, "COSPURI-") {
		return true, nil
	}
	return true, nil
}

func (c *cospuri) normalizeModel(str string) string {
	items := strings.Split(str, "-")
	for i := 0; i < len(items); i++ { //Capitalize each part
		runes := []rune(strings.ToLower(items[i]))
		runes[0] = unicode.ToUpper(runes[0])
		items[i] = string(runes)
	}
	return strings.Join(items, "-")
}

func (c *cospuri) extractModelAndID(number string) (string, string, error) {
	idx := strings.Index(number, "-")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid number format")
	}
	//remove prefix: cospuri
	number = number[idx+1:]

	idx = strings.LastIndex(number, "-")
	//v2 format
	if idx < 0 {
		if !defaultCosPuriV2NumberFormatRegexp.MatchString(number) {
			return "", "", fmt.Errorf("invalid v2 format:%s", number)
		}
		return "", number, nil
	}
	//v1 format
	model := number[:idx]
	id := number[idx+1:]
	if !defaultCosPuriV1NumberFormatRegexp.MatchString(id) {
		return "", "", fmt.Errorf("invalid v1 format:%s", id)
	}

	return c.normalizeModel(model), id, nil
}

func (c *cospuri) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	//返回一个假地址, 仅包含hostname
	return http.NewRequestWithContext(ctx, http.MethodGet, api.MustSelectDomain(defaultCosPuriHosts), nil)
}

func (c *cospuri) handleHTTPRequestForV1Format(ctx context.Context,
	invoker api.HTTPInvoker, originReq *http.Request, model, id string) (*http.Response, string, error) {

	id = strings.ToLower(id)
	uri := fmt.Sprintf("%s://%s/model/%s", originReq.URL.Scheme, originReq.URL.Host, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, "", err
	}
	realNumberId := ""
	rsp, err := twostep.HandleXPathTwoStepSearch(ctx, invoker, req, &twostep.XPathTwoStepContext{
		Ps: []*twostep.XPathPair{
			{
				Name:  "fetch_id_list",
				XPath: `//div[@class="scene-thumb aspect-16_9"]/a/@href`,
			},
		},
		LinkSelector: func(ps []*twostep.XPathPair) (string, bool, error) {
			if len(ps) == 0 {
				return "", false, fmt.Errorf("no id list found")
			}
			for _, item := range ps[0].Result {
				uri, err := url.ParseRequestURI(item)
				if err != nil {
					logutil.GetLogger(ctx).Warn("unable to parse request uri, decode failed, may be xpath content changed in remote site", zap.Error(err), zap.String("item", item))
					continue
				}
				sampleId := strings.ToLower(uri.Query().Get("id"))
				if len(id) == 0 {
					logutil.GetLogger(ctx).Warn("unable to parse request uri, id not found", zap.String("item", item))
					continue
				}
				if len(sampleId) != 0 && strings.HasPrefix(sampleId, id) {
					realNumberId = sampleId
					return item, true, nil
				}
			}
			return "", false, nil
		},
		ValidStatusCode:       []int{http.StatusOK},
		CheckResultCountMatch: false,
		LinkPrefix:            fmt.Sprintf("%s://%s", originReq.URL.Scheme, originReq.URL.Host),
	})
	if err != nil {
		return nil, "", err
	}
	return rsp, realNumberId, nil
}

func (c *cospuri) handleHTTPRequestForV2Format(ctx context.Context,
	invoker api.HTTPInvoker, originReq *http.Request, _, id string) (*http.Response, string, error) {
	uri := fmt.Sprintf("%s://%s/sample?id=%s", originReq.URL.Scheme, originReq.URL.Host, strings.ToLower(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, "", err
	}
	rsp, err := invoker(ctx, req)
	if err != nil {
		return nil, "", err
	}
	return rsp, id, nil
}

func (c *cospuri) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, originReq *http.Request) (*http.Response, error) {
	model, id, err := c.extractModelAndID(meta.GetNumberId(ctx))
	if err != nil {
		return nil, err
	}
	isV2Fmt := len(model) == 0
	logutil.GetLogger(ctx).Debug("decode model and id succ", zap.Bool("is_v2_format", isV2Fmt), zap.String("model", model), zap.String("id", id))
	requestHandler := c.handleHTTPRequestForV1Format
	if isV2Fmt {
		requestHandler = c.handleHTTPRequestForV2Format
	}
	rsp, realid, err := requestHandler(ctx, invoker, originReq, model, id)
	if err != nil {
		return nil, err
	}
	api.SetKeyValue(ctx, defaultCospuriRealNumberIdKey, realid)
	return rsp, nil
}

func (c *cospuri) extractCoverUrl(in string) string {
	//format: background:url(https://aaaaa.cccc/preview/04264arm/scene-lg.jpg
	in = strings.ReplaceAll(in, " ", "")
	matches := defaultCosPuriCoverMatchRegexp.FindStringSubmatch(in)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (c *cospuri) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          ``,
		TitleExpr:           `//div[@class="sample-details"]//div[@class="description"]/text()`, //没有title, 拿desc先顶着= =。
		PlotExpr:            `//div[@class="sample-details"]//div[@class="description"]/text()`,
		ActorListExpr:       `//div[@class="sample-details"]//div[@class="sample-model"]/a/text()`,
		ReleaseDateExpr:     "",
		DurationExpr:        `//div[@class="sample-details"]//div[@class="detail-box"]/div[@class="length"]/strong/text()`,
		StudioExpr:          "",
		LabelExpr:           "",
		DirectorExpr:        "",
		SeriesExpr:          "",
		GenreListExpr:       `//div[@class="sample-details"]//a[@class="tag"]/text()`,
		CoverExpr:           `//div[@class="main wide"]/div/div[@class="player fp-slim fp-edgy fp-mute"]/@style`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="sample-left"]/div[@class="thumb"]/a/@href`,
	}
	mm, err := dec.DecodeHTML(data,
		decoder.WithDurationParser(
			parser.DefaultMMDurationParser(ctx),
		),
		decoder.WithCoverParser(c.extractCoverUrl),
	)
	if err != nil {
		return nil, false, err
	}
	realid, ok := api.GetKeyValue(ctx, defaultCospuriRealNumberIdKey)
	if !ok {
		return nil, false, fmt.Errorf("cospuri real id not found")
	}
	mm.Number = strings.ToUpper("cospuri-" + realid)
	mm.SwithConfig.DisableNumberReplace = true
	mm.SwithConfig.DisableReleaseDateCheck = true
	mm.TitleLang = enum.MetaLangEn
	mm.PlotLang = enum.MetaLangEn
	mm.GenresLang = enum.MetaLangEn
	return mm, true, nil
}

func init() {
	factory.Register(constant.SSCospuri, factory.PluginToCreator(&cospuri{}))
}
