package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/xxxsen/yamdc/internal/model"
)

var errNoImpl = errors.New("no impl")

type DefaultPlugin struct{}

func (p *DefaultPlugin) OnGetHosts(_ context.Context) []string {
	return []string{}
}

func (p *DefaultPlugin) OnPrecheckRequest(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (p *DefaultPlugin) OnMakeHTTPRequest(_ context.Context, _ string) (*http.Request, error) {
	return nil, errNoImpl
}

func (p *DefaultPlugin) OnDecorateRequest(_ context.Context, _ *http.Request) error {
	return nil
}

func (p *DefaultPlugin) OnPrecheckResponse(_ context.Context, _ *http.Request, rsp *http.Response) (bool, error) {
	if rsp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return true, nil
}

func (p *DefaultPlugin) OnHandleHTTPRequest(ctx context.Context, invoker HTTPInvoker, req *http.Request) (
	*http.Response,
	error,
) {
	return invoker(ctx, req)
}

func (p *DefaultPlugin) OnDecodeHTTPData(_ context.Context, _ []byte) (*model.MovieMeta, bool, error) {
	return nil, false, errNoImpl
}

func (p *DefaultPlugin) OnDecorateMediaRequest(_ context.Context, _ *http.Request) error {
	return nil
}
