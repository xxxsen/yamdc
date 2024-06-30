package searcher

import (
	"av-capture/model"
	"av-capture/searcher/meta"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type OnHTTPClientInitFunc func(client *http.Client) *http.Client
type OnMakeRequestFunc func(number string) string
type OnDecorateRequestFunc func(req *http.Request) error
type OnDecodeHTTPDataFunc func(data []byte) (*meta.AvMeta, error)
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

func (p *DefaultSearcher) Search(number string) (*model.AvMeta, error) {
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
	os.WriteFile(fmt.Sprintf("test_data_%s", number), data, 0666)
	meta, err := p.opt.OnDecodeHTTPData(data)
	if err != nil {
		return nil, fmt.Errorf("decode http data failed, err:%w", err)
	}
	if err := p.verifyMeta(meta); err != nil {
		return nil, fmt.Errorf("verify meta failed, err:%w", err)
	}
	p.fixMeta(req, meta)
	return p.renderAvMetaToModelAvMeta(meta), nil
}

func (p *DefaultSearcher) verifyMeta(meta *meta.AvMeta) error {
	if len(meta.Cover) == 0 {
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

func (p *DefaultSearcher) fixMeta(req *http.Request, meta *meta.AvMeta) {
	prefix := req.URL.Scheme + "://" + req.URL.Host
	p.fixSingleURL(&meta.Cover, prefix)
	p.fixSingleURL(&meta.Poster, prefix)
	for i := 0; i < len(meta.SampleImages); i++ {
		p.fixSingleURL(&meta.SampleImages[i], prefix)
	}
}

func (p *DefaultSearcher) fixSingleURL(input *string, prefix string) {
	if strings.HasPrefix(*input, "/") {
		*input = prefix + *input
	}
}

func (p *DefaultSearcher) renderAvMetaToModelAvMeta(in *meta.AvMeta) *model.AvMeta {
	out := &model.AvMeta{
		Number:      in.Number,
		Title:       in.Title,
		Actors:      in.Actors,
		ReleaseDate: in.ReleaseDate,
		Duration:    in.Duration,
		Studio:      in.Studio,
		Label:       in.Label,
		Series:      in.Series,
		Genres:      in.Genres,
	}
	images := make([]string, 0, len(in.SampleImages)+2)
	images = append(images, in.Cover, in.Poster)
	images = append(images, in.SampleImages...)
	imageDataList := p.fetchImageDatas(images)
	out.Cover = imageDataList[0]
	out.Poster = imageDataList[1]
	imageDataList = imageDataList[2:]
	for _, item := range imageDataList {
		if item == nil {
			continue
		}
		out.SampleImages = append(out.SampleImages, item)
	}
	return out
}

func (p *DefaultSearcher) fetchImageDatas(urls []string) []*model.Image {
	rs := make([]*model.Image, len(urls))
	for idx, url := range urls {
		if len(url) == 0 {
			continue
		}
		data, err := p.fetchImageData(url)
		if err != nil {
			logutil.GetLogger(context.Background()).Error("fetch image data failed", zap.Error(err), zap.String("url", url))
			continue
		}
		rs[idx] = data
	}
	return rs
}

func (p *DefaultSearcher) fetchImageData(url string) (*model.Image, error) {
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
	dst := &model.Image{
		Name: url,
		Data: data,
	}
	return dst, nil
}
