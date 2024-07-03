package plugin

import (
	"av-capture/model"
	"context"
	"fmt"
	"net/http"
)

type PluginContext struct {
	ctx    context.Context
	attach map[string]interface{}
}

func NewPluginContext(ctx context.Context) *PluginContext {
	return &PluginContext{
		ctx:    ctx,
		attach: make(map[string]interface{}),
	}
}

func (s *PluginContext) GetContext() context.Context {
	return s.ctx
}

func (s *PluginContext) SetKey(key string, val interface{}) {
	s.attach[key] = val
}

func (s *PluginContext) GetKey(key string) (interface{}, bool) {
	v, ok := s.attach[key]
	return v, ok
}

func (s *PluginContext) GetKeyOrDefault(key string, def interface{}) interface{} {
	if v, ok := s.GetKey(key); ok {
		return v
	}
	return def
}

type IPlugin interface {
	OnPrecheck(ctx *PluginContext, number string) (bool, error)
	OnHTTPClientInit(client *http.Client) *http.Client
	OnMakeHTTPRequest(ctx *PluginContext, number string) (*http.Request, error)
	OnDecorateRequest(ctx *PluginContext, req *http.Request) error
	OnHandleHTTPRequest(ctx *PluginContext, client *http.Client, req *http.Request) (*http.Response, error)
	OnPrecheckIsSearchSucc(ctx *PluginContext, req *http.Request, rsp *http.Response) (bool, error)
	OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error)
	OnDecorateMediaRequest(ctx *PluginContext, req *http.Request) error
}

var _ IPlugin = &DefaultPlugin{}

type DefaultPlugin struct {
}

func (p *DefaultPlugin) OnPrecheck(ctx *PluginContext, number string) (bool, error) {
	return true, nil
}

func (p *DefaultPlugin) OnHTTPClientInit(client *http.Client) *http.Client {
	return client
}

func (p *DefaultPlugin) OnMakeHTTPRequest(ctx *PluginContext, number string) (*http.Request, error) {
	return nil, fmt.Errorf("no impl")
}

func (p *DefaultPlugin) OnDecorateRequest(ctx *PluginContext, req *http.Request) error {
	p.defaultDecorate(ctx, req)
	return nil
}

func (p *DefaultPlugin) OnPrecheckIsSearchSucc(ctx *PluginContext, req *http.Request, rsp *http.Response) (bool, error) {
	if rsp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return true, nil
}

func (p *DefaultPlugin) OnHandleHTTPRequest(ctx *PluginContext, client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}

func (p *DefaultPlugin) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
	return nil, false, fmt.Errorf("no impl")
}

func (p *DefaultPlugin) OnDecorateMediaRequest(ctx *PluginContext, req *http.Request) error {
	p.defaultDecorate(ctx, req)
	return nil
}

func (p *DefaultPlugin) defaultDecorate(_ *PluginContext, req *http.Request) {
	if len(req.UserAgent()) == 0 {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0")
	}
	if len(req.Referer()) == 0 {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", req.URL.Scheme, req.URL.Host))
	}
}
