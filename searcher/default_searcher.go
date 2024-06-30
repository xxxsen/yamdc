package searcher

import (
	"av-capture/model"
	"av-capture/option"
	"av-capture/store"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type OnHTTPClientInitFunc func(client *http.Client) *http.Client
type OnMakeRequestFunc func(number string) string
type OnDecorateRequestFunc func(req *http.Request) error
type OnDecodeHTTPDataFunc func(data []byte) (*model.AvMeta, error)
type OnDecorateMediaRequestFunc func(req *http.Request) error

type DefaultSearcher struct {
	name   string
	client *http.Client
	opt    *DefaultSearchOption
}

type DefaultSearchOption struct {
	OnHTTPClientInit       OnHTTPClientInitFunc
	OnMakeRequest          OnMakeRequestFunc
	OnDecorateRequest      OnDecorateRequestFunc
	OnDecodeHTTPData       OnDecodeHTTPDataFunc
	OnDecorateMediaRequest OnDecorateMediaRequestFunc
}

func NewDefaultSearcher(name string, opt *DefaultSearchOption) (ISearcher, error) {
	if opt == nil {
		return nil, fmt.Errorf("invalid plugin opt")
	}
	if opt.OnMakeRequest == nil {
		return nil, fmt.Errorf("invalid make request func")
	}
	if opt.OnDecodeHTTPData == nil {
		return nil, fmt.Errorf("invalid decode http data func")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	if client.Jar == nil {
		client.Jar = jar
	}
	if opt.OnHTTPClientInit != nil {
		client = opt.OnHTTPClientInit(client)
	}
	return &DefaultSearcher{
		name:   name,
		client: client,
		opt:    opt,
	}, nil
}

func (p *DefaultSearcher) Name() string {
	return p.name
}

func (p *DefaultSearcher) setDefaultHttpOptions(req *http.Request) error {
	if len(req.UserAgent()) == 0 {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0")
	}
	if len(req.Referer()) == 0 {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", req.URL.Scheme, req.URL.Host))
	}
	return nil
}

func (p *DefaultSearcher) getResponseBody(rsp *http.Response) (io.ReadCloser, error) {
	switch rsp.Header.Get("Content-Encoding") {
	case "gzip":
		return gzip.NewReader(rsp.Body)
	default:
		return rsp.Body, nil
	}
}

func (p *DefaultSearcher) decorateRequest(req *http.Request) error {
	if p.opt.OnDecorateRequest != nil {
		if err := p.opt.OnDecorateRequest(req); err != nil {
			return err
		}
	}
	if err := p.setDefaultHttpOptions(req); err != nil {
		return err
	}
	return nil
}

func (p *DefaultSearcher) decorateImageRequest(req *http.Request) error {
	if p.opt.OnDecorateMediaRequest != nil {
		if err := p.opt.OnDecorateMediaRequest(req); err != nil {
			return err
		}
	}
	if err := p.setDefaultHttpOptions(req); err != nil {
		return err
	}
	return nil
}

func (p *DefaultSearcher) buildMetaCacheKey(number string) string {
	return fmt.Sprintf("avc:search:cache:%s:%s", p.name, strings.ToUpper(number))
}

func (p *DefaultSearcher) readMetaFromCache(number string) (*model.AvMeta, error) {
	raw, err := store.GetDefault().GetData(p.buildMetaCacheKey(number))
	if err != nil {
		return nil, err
	}
	meta := &model.AvMeta{}
	if err := json.Unmarshal(raw, meta); err != nil {
		return nil, err
	}
	logutil.GetLogger(context.Background()).Debug("read meta from cache succ", zap.String("number", number))
	return meta, nil
}

func (p *DefaultSearcher) writeMetaToCache(number string, meta *model.AvMeta) {
	key := p.buildMetaCacheKey(number)
	raw, err := json.Marshal(meta)
	if err != nil {
		return
	}
	_ = store.GetDefault().PutWithNamingKey(key, raw)
}

func (p *DefaultSearcher) Search(number string) (*model.AvMeta, error) {
	if option.GetSwitchConfig().EnableMetaCache {
		if m, err := p.readMetaFromCache(number); err == nil {
			return m, nil
		}
	}

	uri := p.opt.OnMakeRequest(number)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("make http request failed, err:%w", err)
	}
	if err := p.decorateRequest(req); err != nil {
		return nil, fmt.Errorf("decorate request failed, err:%w", err)
	}
	rsp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed, err:%w", err)
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid http status code:%d", rsp.StatusCode)
	}
	defer rsp.Body.Close()
	reader, err := p.getResponseBody(rsp)
	if err != nil {
		return nil, fmt.Errorf("determine reader failed, err:%w", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read body failed, err:%w", err)
	}
	meta, err := p.opt.OnDecodeHTTPData(data)
	if err != nil {
		return nil, fmt.Errorf("decode http data failed, err:%w", err)
	}
	//重建不规范的元数据
	p.fixMeta(req, meta)
	//将远程数据保存到本地, 并替换文件key
	p.storeImageData(meta)
	if err := p.verifyMeta(meta); err != nil {
		return nil, fmt.Errorf("verify meta failed, err:%w", err)
	}
	p.writeMetaToCache(number, meta)
	return meta, nil
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
	p.fixSingleURL(&meta.Cover.Name, prefix)
	p.fixSingleURL(&meta.Poster.Name, prefix)
	for i := 0; i < len(meta.SampleImages); i++ {
		p.fixSingleURL(&meta.SampleImages[i].Name, prefix)
	}
}

func (p *DefaultSearcher) fixSingleURL(input *string, prefix string) {
	if strings.HasPrefix(*input, "/") {
		*input = prefix + *input
	}
}

func (p *DefaultSearcher) storeImageData(in *model.AvMeta) {
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
	imageDataMap := p.saveRemoteURLData(images)
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

func (p *DefaultSearcher) saveRemoteURLData(urls []string) map[string]string {
	rs := make(map[string]string, len(urls))
	for _, url := range urls {
		if len(url) == 0 {
			continue
		}
		logger := logutil.GetLogger(context.Background()).With(zap.String("url", url))
		key := p.buildURLCacheKey(url)
		if option.GetSwitchConfig().EnableMediaCache && store.GetDefault().IsCacheExist(key) {
			rs[url] = key
			continue
		}
		data, err := p.fetchImageData(url)
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

func (p *DefaultSearcher) fetchImageData(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("make request for url:%s failed, err:%w", url, err)
	}
	if err := p.decorateImageRequest(req); err != nil {
		return nil, fmt.Errorf("decode request failed, err:%w", err)
	}
	rsp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get url data failed, err:%w", err)
	}

	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get url data http code not ok, code:%d", rsp.StatusCode)
	}
	reader, err := p.getResponseBody(rsp)
	if err != nil {
		return nil, fmt.Errorf("get response body failed, err:%w", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read url data failed, err:%w", err)
	}
	return data, nil
}
