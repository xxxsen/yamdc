package searcher

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/plugin"
	"yamdc/searcher/utils"
	"yamdc/store"
	"yamdc/useragent"

	"github.com/imroc/req/v3"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type DefaultSearcher struct {
	name    string
	ua      string
	invoker plugin.HTTPInvoker
	plg     plugin.IPlugin
}

func MustNewDefaultSearcher(name string, plg plugin.IPlugin) ISearcher {
	s, err := NewDefaultSearcher(name, plg)
	if err != nil {
		panic(err)
	}
	return s
}

func defaultInvoker() plugin.HTTPInvoker {
	client := req.NewClient()
	client.ImpersonateChrome()
	client.SetRedirectPolicy(req.SameDomainRedirectPolicy())
	return func(ctx *plugin.PluginContext, req *http.Request) (*http.Response, error) {
		return client.RoundTrip(req)
	}
}

func NewDefaultSearcher(name string, plg plugin.IPlugin) (ISearcher, error) {
	invoker := plg.OnHTTPClientInit()
	if invoker == nil {
		invoker = defaultInvoker()
	}
	ss := &DefaultSearcher{
		name:    name,
		invoker: invoker,
		plg:     plg,
		ua:      useragent.Select(),
	}
	return ss, nil
}

func (p *DefaultSearcher) Name() string {
	return p.name
}

func (p *DefaultSearcher) setDefaultHttpOptions(req *http.Request) error {
	if len(req.UserAgent()) == 0 {
		req.Header.Set("User-Agent", p.ua)
	}
	if len(req.Referer()) == 0 {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", req.URL.Scheme, req.URL.Host))
	}
	return nil
}

func (p *DefaultSearcher) decorateRequest(ctx *plugin.PluginContext, req *http.Request) error {
	if err := p.plg.OnDecorateRequest(ctx, req); err != nil {
		return err
	}
	if err := p.setDefaultHttpOptions(req); err != nil {
		return err
	}
	return nil
}

func (p *DefaultSearcher) decorateImageRequest(ctx *plugin.PluginContext, req *http.Request) error {
	if err := p.plg.OnDecorateMediaRequest(ctx, req); err != nil {
		return err
	}
	if err := p.setDefaultHttpOptions(req); err != nil {
		return err
	}
	return nil
}

func (p *DefaultSearcher) invokeHTTPRequest(ctx *plugin.PluginContext, req *http.Request) (*http.Response, error) {
	if err := p.decorateRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("decorate request failed, err:%w", err)
	}
	return p.invoker(ctx, req)
}

func (p *DefaultSearcher) Search(ctx context.Context, number *number.Number) (*model.AvMeta, bool, error) {
	pctx := plugin.NewPluginContext(ctx)
	ok, err := p.plg.OnPrecheckRequest(pctx, number)
	if err != nil {
		return nil, false, fmt.Errorf("precheck failed, err:%w", err)
	}
	if !ok {
		return nil, false, nil
	}
	req, err := p.plg.OnMakeHTTPRequest(pctx, number)
	if err != nil {
		return nil, false, fmt.Errorf("make http request failed, err:%w", err)
	}
	rsp, err := p.plg.OnHandleHTTPRequest(pctx, p.invokeHTTPRequest, req)
	if err != nil {
		return nil, false, fmt.Errorf("do request failed, err:%w", err)
	}
	isSearchSucc, err := p.plg.OnPrecheckResponse(pctx, req, rsp)
	if err != nil {
		return nil, false, fmt.Errorf("precheck responnse failed, err:%w", err)
	}
	if !isSearchSucc {
		return nil, false, nil
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("invalid http status code:%d", rsp.StatusCode)
	}
	defer rsp.Body.Close()
	data, err := utils.ReadHTTPData(rsp)
	if err != nil {
		return nil, false, fmt.Errorf("read body failed, err:%w", err)
	}
	meta, decodeSucc, err := p.plg.OnDecodeHTTPData(pctx, data)
	if err != nil {
		return nil, false, fmt.Errorf("decode http data failed, err:%w", err)
	}
	if !decodeSucc {
		return nil, false, nil
	}
	//重建不规范的元数据
	p.fixMeta(req, meta)
	//将远程数据保存到本地, 并替换文件key
	p.storeImageData(pctx, meta)
	if err := p.verifyMeta(meta); err != nil {
		logutil.GetLogger(ctx).Error("verify meta not pass, treat as not found", zap.Error(err), zap.String("plugin", p.name))
		return nil, false, nil
	}
	meta.ExtInfo.ScrapeSource = p.name
	return meta, true, nil
}

func (p *DefaultSearcher) verifyMeta(meta *model.AvMeta) error {
	if meta.Cover == nil || len(meta.Cover.Name) == 0 {
		return fmt.Errorf("no cover")
	}
	if len(meta.Number) == 0 {
		return fmt.Errorf("no number")
	}
	if len(meta.Title) == 0 {
		return fmt.Errorf("no title")
	}
	return nil
}

func (p *DefaultSearcher) fixMeta(req *http.Request, meta *model.AvMeta) {
	meta.Number = strings.ToUpper(meta.Number)
	prefix := req.URL.Scheme + "://" + req.URL.Host
	if meta.Cover != nil {
		p.fixSingleURL(req, &meta.Cover.Name, prefix)
	}
	if meta.Poster != nil {
		p.fixSingleURL(req, &meta.Poster.Name, prefix)
	}
	for i := 0; i < len(meta.SampleImages); i++ {
		p.fixSingleURL(req, &meta.SampleImages[i].Name, prefix)
	}
}

func (p *DefaultSearcher) fixSingleURL(req *http.Request, input *string, prefix string) {
	if strings.HasPrefix(*input, "//") {
		*input = req.URL.Scheme + ":" + *input
		return
	}
	if strings.HasPrefix(*input, "/") {
		*input = prefix + *input
		return
	}
}

func (p *DefaultSearcher) storeImageData(ctx *plugin.PluginContext, in *model.AvMeta) {
	images := make([]string, 0, len(in.SampleImages)+2)
	if in.Cover != nil {
		images = append(images, in.Cover.Name)
	}
	if in.Poster != nil {
		images = append(images, in.Poster.Name)
	}
	for _, item := range in.SampleImages {
		images = append(images, item.Name)
	}
	imageDataMap := p.saveRemoteURLData(ctx, images)
	if in.Cover != nil {
		in.Cover.Key = imageDataMap[in.Cover.Name]
		//如果没有成功下载到数据, 那么直接置空
		if len(in.Cover.Key) == 0 {
			in.Cover = nil
		}
	}
	if in.Poster != nil {
		in.Poster.Key = imageDataMap[in.Poster.Name]
		if len(in.Poster.Key) == 0 {
			in.Poster = nil
		}
	}
	rebuildSampleList := make([]*model.File, 0, len(in.SampleImages))
	for _, item := range in.SampleImages {
		item.Key = imageDataMap[item.Name]
		rebuildSampleList = append(rebuildSampleList, item)
	}
	in.SampleImages = rebuildSampleList
}

func (p *DefaultSearcher) saveRemoteURLData(ctx *plugin.PluginContext, urls []string) map[string]string {
	rs := make(map[string]string, len(urls))
	for _, url := range urls {
		if len(url) == 0 {
			continue
		}
		logger := logutil.GetLogger(context.Background()).With(zap.String("url", url))
		key := p.buildURLCacheKey(url)
		if store.GetDefault().IsCacheExist(key) {
			rs[url] = key
			continue
		}
		data, err := p.fetchImageData(ctx, url)
		if err != nil {
			logger.Error("fetch image data failed", zap.Error(err))
			continue
		}
		err = store.GetDefault().PutWithNamingKey(key, data)
		if err != nil {
			logger.Error("put image data to store failed", zap.Error(err))
		}
		rs[url] = key
	}
	return rs
}

func (p *DefaultSearcher) buildURLCacheKey(url string) string {
	h := md5.New()
	_, _ = h.Write([]byte(url))
	hashsum := hex.EncodeToString(h.Sum(nil))
	key := fmt.Sprintf("avc:search:cache:url:%s:%s", p.name, hashsum)
	return key
}

func (p *DefaultSearcher) fetchImageData(ctx *plugin.PluginContext, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("make request for url:%s failed, err:%w", url, err)
	}
	if err := p.decorateImageRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("decode request failed, err:%w", err)
	}
	rsp, err := p.invoker(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get url data failed, err:%w", err)
	}

	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get url data http code not ok, code:%d", rsp.StatusCode)
	}
	data, err := utils.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read url data failed, err:%w", err)
	}
	return data, nil
}
