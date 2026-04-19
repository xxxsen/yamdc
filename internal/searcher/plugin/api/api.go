package api

import (
	"context"
	"net/http"

	"github.com/xxxsen/yamdc/internal/model"
)

type HTTPInvoker func(ctx context.Context, req *http.Request) (*http.Response, error)

// IPlugin 是 Searcher 抓取插件的完整生命周期 hook 接口.
// 拆成多个小接口意义不大: 每个方法都由同一个插件实现(yaml / twostep / multilink)
// 按抓取流水线顺序回调, 是一条"生产线"而不是能力集合 — 拆分反而让调用方需要
// 同时持有多把钥匙并做一次转型才能完整触发流水线.
//
//nolint:interfacebloat // pipeline hooks, splitting increases coupling without payoff
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
