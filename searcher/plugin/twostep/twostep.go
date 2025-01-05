package twostep

import (
	"context"
	"fmt"
	"net/http"
	"yamdc/searcher/decoder"
	"yamdc/searcher/plugin/api"
	"yamdc/searcher/utils"
)

type XPathPair struct {
	Name   string
	XPath  string
	Result []string
}

type XPathTwoStepContext struct {
	Ps                    []*XPathPair
	LinkSelector          OnTwoStepLinkSelect
	ValidStatusCode       []int
	CheckResultCountMatch bool
	LinkPrefix            string
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

func HandleXPathTwoStepSearch(ctx context.Context, invoker api.HTTPInvoker, req *http.Request, xctx *XPathTwoStepContext) (*http.Response, error) {
	rsp, err := invoker(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("step search failed, err:%w", err)
	}
	if !isCodeInValidStatusCodeList(xctx.ValidStatusCode, rsp.StatusCode) {
		return nil, fmt.Errorf("status code:%d not in valid list", rsp.StatusCode)
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
				return nil, fmt.Errorf("result count not match, idx:%d, count:%d not match to idx:0, count:%d", i, len(xctx.Ps[i].Result), len(xctx.Ps[0].Result))
			}
		}
		if len(xctx.Ps[0].Result) == 0 {
			return nil, fmt.Errorf("no result found")
		}
	}
	link, ok, err := xctx.LinkSelector(xctx.Ps)
	if err != nil {
		return nil, fmt.Errorf("select link from result failed, err:%w", err)
	}
	if !ok {
		return nil, fmt.Errorf("no link select result found")
	}
	link = xctx.LinkPrefix + link
	req, err = http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return nil, fmt.Errorf("step re-create result page link failed, err:%w", err)
	}
	return invoker(ctx, req)
}
