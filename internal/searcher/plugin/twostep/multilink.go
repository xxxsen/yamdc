package twostep

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

type MultiLinkContext struct {
	ReqBuilder      MultiLinkBuildRequestFunc //用于重建请求的函数
	Numbers         []string                  //用户传入的多个番号, 基于这些番号, 逐个调用ReqBuilder构建链接并请求
	ValidStatusCode []int                     //哪些http状态码是有效的
	ResultTester    OnMultiLinkResultTest     //回调用户函数，确认哪些结果是符合预期的
}

type MultiLinkBuildRequestFunc func(nid string) (*http.Request, error)
type OnMultiLinkResultTest func(raw []byte) (bool, error)

func HandleMultiLinkSearch(ctx context.Context, invoker api.HTTPInvoker, xctx *MultiLinkContext) (*http.Response, error) {
	for _, number := range xctx.Numbers {
		req, err := xctx.ReqBuilder(number)
		if err != nil {
			return nil, fmt.Errorf("build request failed, err:%w", err)
		}
		rsp, err := invoker(ctx, req)
		if err != nil {
			return rsp, fmt.Errorf("step search failed, err:%w", err)
		}
		if !isCodeInValidStatusCodeList(xctx.ValidStatusCode, rsp.StatusCode) {
			_ = rsp.Body.Close()
			continue
		}
		raw, err := client.ReadHTTPData(rsp)
		if err != nil {
			return rsp, fmt.Errorf("step read data as html node failed, err:%w", err)
		}
		ok, err := xctx.ResultTester(raw)
		if err != nil {
			return rsp, fmt.Errorf("test result failed, err:%w", err)
		}
		if !ok {
			continue
		}
		//将body重新设置回去
		rsp.Body = io.NopCloser(bytes.NewReader(raw))
		return rsp, nil
	}
	return nil, fmt.Errorf("no valid result found")
}
