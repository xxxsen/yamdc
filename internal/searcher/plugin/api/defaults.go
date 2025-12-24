package api

import (
	"context"
	"fmt"
	"net/http"
	"github.com/xxxsen/yamdc/internal/model"
)

type DefaultPlugin struct {
}

func (p *DefaultPlugin) OnGetHosts(ctx context.Context) []string {
	return []string{}
}

func (p *DefaultPlugin) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	return true, nil
}

func (p *DefaultPlugin) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	return nil, fmt.Errorf("no impl")
}

func (p *DefaultPlugin) OnDecorateRequest(ctx context.Context, req *http.Request) error {
	return nil
}

func (p *DefaultPlugin) OnPrecheckResponse(ctx context.Context, req *http.Request, rsp *http.Response) (bool, error) {
	if rsp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return true, nil
}

func (p *DefaultPlugin) OnHandleHTTPRequest(ctx context.Context, invoker HTTPInvoker, req *http.Request) (*http.Response, error) {
	return invoker(ctx, req)
}

func (p *DefaultPlugin) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	return nil, false, fmt.Errorf("no impl")
}

func (p *DefaultPlugin) OnDecorateMediaRequest(ctx context.Context, req *http.Request) error {
	return nil
}
