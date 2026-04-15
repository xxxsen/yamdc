package searcher

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

type mockHTTPClient struct {
	do func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.do != nil {
		return m.do(req)
	}
	return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
}

type fullPlugin struct {
	api.DefaultPlugin
	hosts              []string
	precheckOK         bool
	precheckErr        error
	makeReqFn          func(ctx context.Context, number string) (*http.Request, error)
	decodeData         *model.MovieMeta
	decodeOK           bool
	decodeErr          error
	decorateReqErr     error
	decorateMediaErr   error
	precheckRspOK      bool
	precheckRspErr     error
	handleHTTPReqFn    func(ctx context.Context, invoker api.HTTPInvoker, req *http.Request) (*http.Response, error)
}

func (p *fullPlugin) OnGetHosts(_ context.Context) []string { return p.hosts }
func (p *fullPlugin) OnPrecheckRequest(_ context.Context, _ string) (bool, error) {
	return p.precheckOK, p.precheckErr
}
func (p *fullPlugin) OnMakeHTTPRequest(ctx context.Context, num string) (*http.Request, error) {
	if p.makeReqFn != nil {
		return p.makeReqFn(ctx, num)
	}
	return http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/"+num, nil)
}
func (p *fullPlugin) OnDecodeHTTPData(_ context.Context, _ []byte) (*model.MovieMeta, bool, error) {
	return p.decodeData, p.decodeOK, p.decodeErr
}
func (p *fullPlugin) OnDecorateRequest(_ context.Context, _ *http.Request) error {
	return p.decorateReqErr
}
func (p *fullPlugin) OnDecorateMediaRequest(_ context.Context, _ *http.Request) error {
	return p.decorateMediaErr
}
func (p *fullPlugin) OnPrecheckResponse(_ context.Context, _ *http.Request, rsp *http.Response) (bool, error) {
	if p.precheckRspErr != nil {
		return false, p.precheckRspErr
	}
	return p.precheckRspOK, nil
}
func (p *fullPlugin) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, req *http.Request) (*http.Response, error) {
	if p.handleHTTPReqFn != nil {
		return p.handleHTTPReqFn(ctx, invoker, req)
	}
	return invoker(ctx, req)
}

type mockSearcher struct {
	name     string
	searchFn func(ctx context.Context, n *number.Number) (*model.MovieMeta, bool, error)
}

func (m *mockSearcher) Name() string { return m.name }
func (m *mockSearcher) Search(ctx context.Context, n *number.Number) (*model.MovieMeta, bool, error) {
	return m.searchFn(ctx, n)
}
func (m *mockSearcher) Check(_ context.Context) error { return nil }

func mustParseNumber(t *testing.T, s string) *number.Number {
	t.Helper()
	n, err := number.Parse(s)
	require.NoError(t, err)
	return n
}
