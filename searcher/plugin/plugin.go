package plugin

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"yamdc/model"
	"yamdc/number"
)

const (
	defaultNumberInfoKey = "plugin_ctx_number_info"
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

func (s *PluginContext) SetNumberInfo(v *number.Number) {
	s.setKey(defaultNumberInfoKey, v)
}

func (s *PluginContext) MustGetNumberInfo() *number.Number {
	if v, ok := s.getKey(defaultNumberInfoKey); ok {
		return v.(*number.Number)
	}
	panic("number info not found")
}

func (s *PluginContext) setKey(key string, val interface{}) {
	s.attach[key] = val
}

func (s *PluginContext) getKey(key string) (interface{}, bool) {
	v, ok := s.attach[key]
	return v, ok
}

type HTTPInvoker func(ctx *PluginContext, req *http.Request) (*http.Response, error)

type IPlugin interface {
	OnHTTPClientInit() HTTPInvoker
	OnPrecheckRequest(ctx *PluginContext, number *number.Number) (bool, error)
	OnMakeHTTPRequest(ctx *PluginContext, number *number.Number) (*http.Request, error)
	OnDecorateRequest(ctx *PluginContext, req *http.Request) error
	OnHandleHTTPRequest(ctx *PluginContext, invoker HTTPInvoker, req *http.Request) (*http.Response, error)
	OnPrecheckResponse(ctx *PluginContext, req *http.Request, rsp *http.Response) (bool, error)
	OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error)
	OnDecorateMediaRequest(ctx *PluginContext, req *http.Request) error
}

var _ IPlugin = &DefaultPlugin{}

type DefaultPlugin struct {
}

func (p *DefaultPlugin) OnPrecheckRequest(ctx *PluginContext, number *number.Number) (bool, error) {
	return true, nil
}

func (p *DefaultPlugin) OnHTTPClientInit() HTTPInvoker {
	return nil
}

func (p *DefaultPlugin) OnMakeHTTPRequest(ctx *PluginContext, number *number.Number) (*http.Request, error) {
	return nil, fmt.Errorf("no impl")
}

func (p *DefaultPlugin) OnDecorateRequest(ctx *PluginContext, req *http.Request) error {
	return nil
}

func (p *DefaultPlugin) OnPrecheckResponse(ctx *PluginContext, req *http.Request, rsp *http.Response) (bool, error) {
	if rsp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return true, nil
}

func (p *DefaultPlugin) OnHandleHTTPRequest(ctx *PluginContext, invoker HTTPInvoker, req *http.Request) (*http.Response, error) {
	return invoker(ctx, req)
}

func (p *DefaultPlugin) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
	return nil, false, fmt.Errorf("no impl")
}

func (p *DefaultPlugin) OnDecorateMediaRequest(ctx *PluginContext, req *http.Request) error {
	return nil
}

type CreatorFunc func(args interface{}) (IPlugin, error)

var mp = make(map[string]CreatorFunc)

func Register(name string, fn CreatorFunc) {
	mp[name] = fn
}

func CreatePlugin(name string, args interface{}) (IPlugin, error) {
	cr, ok := mp[name]
	if !ok {
		return nil, fmt.Errorf("plugin:%s not found", name)
	}
	return cr(args)
}

func PluginToCreator(plg IPlugin) CreatorFunc {
	return func(args interface{}) (IPlugin, error) {
		return plg, nil
	}
}

func Plugins() []string {
	rs := make([]string, 0, len(mp))
	for k := range mp {
		rs = append(rs, k)
	}
	return sort.StringSlice(rs)
}
