package searcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/hasher"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	errHTTPClientNil     = errors.New("http client is nil")
	errStorageNil        = errors.New("storage is nil")
	errNoDataFound       = errors.New("no data found")
	errInvalidHTTPStatus = errors.New("invalid http status code")
	errNoCover           = errors.New("no cover")
	errNoNumber          = errors.New("no number")
	errNoTitle           = errors.New("no title")
	errNoReleaseDate     = errors.New("no release_date")
	errHTTPCodeNotOK     = errors.New("http code not ok")
	errCheckStatusFailed = errors.New("check request status failed")
)

const (
	defaultPageSearchCacheExpire = 30 * 24 * time.Hour
)

type searchCacheContext struct {
	KvData     map[string]string `json:"kv_data"`
	SearchData string            `json:"search_data"`
}

type DefaultSearcher struct {
	name string
	cc   *config
	plg  api.IPlugin
}

func MustNewDefaultSearcher(name string, plg api.IPlugin) ISearcher {
	s, err := NewDefaultSearcher(name, plg)
	if err != nil {
		panic(err)
	}
	return s
}

func NewDefaultSearcher(name string, plg api.IPlugin, opts ...Option) (ISearcher, error) {
	cc := applyOpts(opts...)
	ss := &DefaultSearcher{
		name: name,
		cc:   cc,
		plg:  plg,
	}
	if ss.cc.cli == nil {
		return nil, errHTTPClientNil
	}
	if ss.cc.storage == nil {
		return nil, errStorageNil
	}
	return ss, nil
}

func (p *DefaultSearcher) loadCacheData(
	ctx context.Context,
	key string,
	expire time.Duration,
	loader func() ([]byte, error),
) ([]byte, error) {
	if raw, err := p.cc.storage.GetData(ctx, key); err == nil {
		return raw, nil
	}
	raw, err := loader()
	if err != nil {
		return nil, err
	}
	if err := p.cc.storage.PutData(ctx, key, raw, expire); err != nil {
		return nil, fmt.Errorf("put cache data: %w", err)
	}
	return raw, nil
}

func (p *DefaultSearcher) Check(ctx context.Context) error {
	hosts := p.plg.OnGetHosts(ctx)
	if len(hosts) == 0 {
		return nil
	}
	for _, host := range hosts {
		if err := p.checkOneRequest(ctx, host); err != nil {
			return fmt.Errorf("check one request failed, host:%s, err:%w", host, err)
		}
	}
	return nil
}

func (p *DefaultSearcher) checkOneRequest(ctx context.Context, host string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host, nil)
	if err != nil {
		return fmt.Errorf("create request failed, err:%w", err)
	}
	if err := p.decorateRequest(ctx, req); err != nil {
		return fmt.Errorf("decorate request failed, err:%w", err)
	}
	rsp, err := p.cc.cli.Do(req)
	if err != nil {
		return fmt.Errorf("do check request network failed, err:%w", err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d, status %s: %w", rsp.StatusCode, rsp.Status, errCheckStatusFailed)
	}
	return nil
}

func (p *DefaultSearcher) Name() string {
	return p.name
}

func (p *DefaultSearcher) setDefaultHTTPOptions(req *http.Request) error {
	if len(req.Referer()) == 0 {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", req.URL.Scheme, req.URL.Host))
	}
	return nil
}

func (p *DefaultSearcher) decorateRequest(ctx context.Context, req *http.Request) error {
	if err := p.plg.OnDecorateRequest(ctx, req); err != nil {
		return fmt.Errorf("decorate request: %w", err)
	}
	return p.setDefaultHTTPOptions(req)
}

func (p *DefaultSearcher) decorateImageRequest(ctx context.Context, req *http.Request) error {
	if err := p.plg.OnDecorateMediaRequest(ctx, req); err != nil {
		return fmt.Errorf("decorate media request: %w", err)
	}
	return p.setDefaultHTTPOptions(req)
}

func (p *DefaultSearcher) invokeHTTPRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := p.decorateRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("decorate request failed, err:%w", err)
	}
	rsp, err := p.cc.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke http request: %w", err)
	}
	return rsp, nil
}

func (p *DefaultSearcher) onRetriveData(ctx context.Context, req *http.Request, number *number.Number) ([]byte, error) {
	key := p.name + ":" + number.GetNumberID() + ":v1"
	dataLoader := func() ([]byte, error) {
		rsp, err := p.plg.OnHandleHTTPRequest(ctx, p.invokeHTTPRequest, req)
		if err != nil {
			return nil, fmt.Errorf("do request failed, err:%w", err)
		}
		isSearchSucc, err := p.plg.OnPrecheckResponse(ctx, req, rsp)
		if err != nil {
			return nil, fmt.Errorf("precheck responnse failed, err:%w", err)
		}
		if !isSearchSucc {
			return nil, errNoDataFound
		}
		if rsp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: %d", errInvalidHTTPStatus, rsp.StatusCode)
		}
		defer func() {
			_ = rsp.Body.Close()
		}()
		data, err := client.ReadHTTPData(rsp)
		if err != nil {
			return nil, fmt.Errorf("read body failed, err:%w", err)
		}
		return data, nil
	}
	if !p.cc.searchCache {
		return dataLoader()
	}
	res, err := p.loadCacheData(ctx, key, defaultPageSearchCacheExpire, func() ([]byte, error) {
		res, err := dataLoader()
		if err != nil {
			return nil, err
		}
		cachectx := &searchCacheContext{
			KvData:     api.ExportContainerData(ctx),
			SearchData: string(res),
		}
		return json.Marshal(cachectx)
	})
	if err != nil {
		return nil, fmt.Errorf("load data from cache failed, err:%w", err)
	}
	cachectx := &searchCacheContext{}
	if err := json.Unmarshal(res, cachectx); err != nil {
		return nil, fmt.Errorf("decode search cache data failed, err:%w", err)
	}
	api.ImportContainerData(ctx, cachectx.KvData)
	return []byte(cachectx.SearchData), nil
}

func (p *DefaultSearcher) Search(ctx context.Context, number *number.Number) (*model.MovieMeta, bool, error) {
	ctx = api.InitContainer(ctx)
	ctx = meta.SetNumberID(ctx, number.GetNumberID())
	ok, err := p.plg.OnPrecheckRequest(ctx, number.GetNumberID())
	if err != nil {
		return nil, false, fmt.Errorf("precheck failed, err:%w", err)
	}
	if !ok {
		return nil, false, nil
	}
	req, err := p.plg.OnMakeHTTPRequest(ctx, number.GetNumberID())
	if err != nil {
		return nil, false, fmt.Errorf("make http request failed, err:%w", err)
	}
	data, err := p.onRetriveData(ctx, req, number)
	if err != nil {
		return nil, false, err
	}
	meta, decodeSucc, err := p.plg.OnDecodeHTTPData(ctx, data)
	if err != nil {
		return nil, false, fmt.Errorf("decode http data failed, err:%w", err)
	}
	if !decodeSucc {
		return nil, false, nil
	}
	// 重建不规范的元数据
	p.fixMeta(ctx, req, meta)
	// 将远程数据保存到本地, 并替换文件key
	p.storeImageData(ctx, meta)
	if err := p.verifyMeta(meta); err != nil {
		logutil.GetLogger(ctx).Error("verify meta not pass, treat as not found",
			zap.Error(err), zap.String("plugin", p.name))
		return nil, false, nil
	}
	meta.ExtInfo.ScrapeInfo.Source = p.name
	meta.ExtInfo.ScrapeInfo.DateTs = time.Now().UnixMilli()
	return meta, true, nil
}

func (p *DefaultSearcher) verifyMeta(meta *model.MovieMeta) error {
	if meta.Cover == nil || len(meta.Cover.Name) == 0 {
		return errNoCover
	}
	if len(meta.Number) == 0 {
		return errNoNumber
	}
	if len(meta.Title) == 0 {
		return errNoTitle
	}
	if !meta.SwithConfig.DisableReleaseDateCheck && meta.ReleaseDate == 0 {
		return errNoReleaseDate
	}
	return nil
}

func (p *DefaultSearcher) fixMeta(ctx context.Context, req *http.Request, mvmeta *model.MovieMeta) {
	if !mvmeta.SwithConfig.DisableNumberReplace {
		mvmeta.Number = meta.GetNumberID(ctx) // 直接替换为已经解析到的影片 ID
	}
	prefix := req.URL.Scheme + "://" + req.URL.Host
	if mvmeta.Cover != nil {
		p.fixSingleURL(req, &mvmeta.Cover.Name, prefix)
	}
	if mvmeta.Poster != nil {
		p.fixSingleURL(req, &mvmeta.Poster.Name, prefix)
	}
	for i := 0; i < len(mvmeta.SampleImages); i++ {
		p.fixSingleURL(req, &mvmeta.SampleImages[i].Name, prefix)
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

func (p *DefaultSearcher) storeImageData(ctx context.Context, in *model.MovieMeta) {
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
		// 如果没有成功下载到数据, 那么直接置空
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

const defaultImageDownloadConcurrency = 2

func (p *DefaultSearcher) saveRemoteURLData(ctx context.Context, urls []string) map[string]string {
	var mu sync.Mutex
	rs := make(map[string]string, len(urls))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(defaultImageDownloadConcurrency)
	for _, u := range urls {
		if len(u) == 0 {
			continue
		}
		g.Go(func() error {
			logger := logutil.GetLogger(gctx).With(zap.String("url", u))
			key := hasher.ToSha1(u)
			if ok, _ := p.cc.storage.IsDataExist(gctx, key); ok {
				mu.Lock()
				rs[u] = key
				mu.Unlock()
				return nil
			}
			data, err := p.fetchImageData(gctx, u)
			if err != nil {
				logger.Error("fetch image data failed", zap.Error(err))
				return nil
			}
			if err := p.cc.storage.PutData(gctx, key, data, 0); err != nil {
				logger.Error("put image data to store failed", zap.Error(err))
			}
			mu.Lock()
			rs[u] = key
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		logutil.GetLogger(ctx).Error("image download errgroup failed", zap.Error(err))
	}
	return rs
}

func (p *DefaultSearcher) fetchImageData(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("make request for url:%s failed, err:%w", url, err)
	}
	if err := p.decorateImageRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("decode request failed, err:%w", err)
	}
	rsp, err := p.cc.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get url data failed, err:%w", err)
	}

	defer func() {
		_ = rsp.Body.Close()
	}()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("code %d: %w", rsp.StatusCode, errHTTPCodeNotOK)
	}
	data, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read url data failed, err:%w", err)
	}
	return data, nil
}
