package api

import (
	"context"
	"net/http"
	"github.com/xxxsen/yamdc/internal/model"
)

type HTTPInvoker func(ctx context.Context, req *http.Request) (*http.Response, error)

type IPlugin interface {
	OnGetHosts(ctx context.Context) []string
	OnPrecheckRequest(ctx context.Context, number string) (bool, error)
	OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error)
	OnDecorateRequest(ctx context.Context, req *http.Request) error
	OnHandleHTTPRequest(ctx context.Context, invoker HTTPInvoker, req *http.Request) (*http.Response, error)
	OnPrecheckResponse(ctx context.Context, req *http.Request, rsp *http.Response) (bool, error)
	OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error)
	OnDecorateMediaRequest(ctx context.Context, req *http.Request) error
}
