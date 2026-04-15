package twostep

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/utils"
)

var (
	errStatusCodeNotInValidList = errors.New("status code not in valid list")
	errResultCountMismatch      = errors.New("result count not match")
	errNoResultFound            = errors.New("no result found")
	errNoLinkSelectResult       = errors.New("no link select result found")
)

type XPathPair struct {
	Name   string
	XPath  string
	Result []string
}

type XPathTwoStepContext struct {
	Ps                    []*XPathPair        // 用户传入多组XPath, 用于在页面获取完数据后进行数据提取
	LinkSelector          OnTwoStepLinkSelect // 回调用户函数，确认哪些链接是符合预期的
	ValidStatusCode       []int               // http的哪些状态码是有效的
	CheckResultCountMatch bool                // 检查多组xpath的result个数是否一致
	LinkPrefix            string              // 用于重建链接的前缀
}

type OnTwoStepLinkSelect func(ps []*XPathPair) (string, bool, error)

func isCodeInValidStatusCodeList(lst []int, code int) bool {
	for _, c := range lst {
		if c == code {
			return true
		}
	}
	return false
}

func HandleXPathTwoStepSearch(
	ctx context.Context,
	invoker api.HTTPInvoker,
	req *http.Request,
	xctx *XPathTwoStepContext,
) (*http.Response, error) {
	rsp, err := invoker(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("step search failed, err:%w", err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	if !isCodeInValidStatusCodeList(xctx.ValidStatusCode, rsp.StatusCode) {
		return nil, fmt.Errorf("status code %d: %w", rsp.StatusCode, errStatusCodeNotInValidList)
	}
	node, err := utils.ReadDataAsHTMLTree(rsp)
	if err != nil {
		return nil, fmt.Errorf("step read data as html node failed, err:%w", err)
	}
	for _, p := range xctx.Ps {
		p.Result = decoder.DecodeList(node, p.XPath)
	}
	if xctx.CheckResultCountMatch {
		for i := 1; i < len(xctx.Ps); i++ {
			if len(xctx.Ps[i].Result) != len(xctx.Ps[0].Result) {
				return nil, fmt.Errorf(
					"idx %d count %d vs idx 0 count %d: %w",
					i, len(xctx.Ps[i].Result), len(xctx.Ps[0].Result), errResultCountMismatch,
				)
			}
		}
		if len(xctx.Ps[0].Result) == 0 {
			return nil, errNoResultFound
		}
	}
	link, ok, err := xctx.LinkSelector(xctx.Ps)
	if err != nil {
		return nil, fmt.Errorf("select link from result failed, err:%w", err)
	}
	if !ok {
		return nil, errNoLinkSelectResult
	}
	link = xctx.LinkPrefix + link
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, fmt.Errorf("step re-create result page link failed, err:%w", err)
	}
	return invoker(ctx, req)
}
