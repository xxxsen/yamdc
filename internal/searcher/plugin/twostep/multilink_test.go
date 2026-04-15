package twostep

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleMultiLinkSearch_ReqBuilderError(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"a"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return nil, fmt.Errorf("build fail")
		},
		ResultTester: func(raw []byte) (bool, error) { return true, nil },
	}
	_, err := HandleMultiLinkSearch(context.Background(), nil, xctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build request")
}

func TestHandleMultiLinkSearch_InvokerError(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"a"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return httptest.NewRequest(http.MethodGet, "http://x/"+nid, nil), nil
		},
		ResultTester: func(raw []byte) (bool, error) { return true, nil },
	}
	invoker := func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial error")
	}
	_, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step search failed")
}

func TestHandleMultiLinkSearch_SkipsBadStatus(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"n1", "n2"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return httptest.NewRequest(http.MethodGet, "http://x/"+nid, nil), nil
		},
		ResultTester: func(raw []byte) (bool, error) {
			return strings.Contains(string(raw), "GOOD"), nil
		},
	}
	calls := 0
	invoker := func(ctx context.Context, req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("no")),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("GOOD")),
		}, nil
	}
	rsp, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	body, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	assert.Equal(t, "GOOD", string(body))
	_ = rsp.Body.Close()
}

func TestHandleMultiLinkSearch_ReadBodyFails(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"n1"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return httptest.NewRequest(http.MethodGet, "http://x/"+nid, nil), nil
		},
		ResultTester: func(raw []byte) (bool, error) { return true, nil },
	}
	invoker := func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errReader{}),
		}, nil
	}
	_, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read data as html")
}

type errReader struct{}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read err")
}

func TestHandleMultiLinkSearch_ResultTesterError(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"n1"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return httptest.NewRequest(http.MethodGet, "http://x/"+nid, nil), nil
		},
		ResultTester: func(raw []byte) (bool, error) {
			return false, fmt.Errorf("tester boom")
		},
	}
	invoker := func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("data")),
		}, nil
	}
	_, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test result failed")
}

func TestHandleMultiLinkSearch_ResultTesterFalseThenSuccess(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"n1", "n2"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return httptest.NewRequest(http.MethodGet, "http://x/"+nid, nil), nil
		},
		ResultTester: func(raw []byte) (bool, error) {
			return strings.Contains(string(raw), "WIN"), nil
		},
	}
	calls := 0
	invoker := func(ctx context.Context, req *http.Request) (*http.Response, error) {
		calls++
		body := "no"
		if calls == 2 {
			body = "WIN"
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
	rsp, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	require.NoError(t, err)
	data, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	assert.Equal(t, "WIN", string(data))
	_ = rsp.Body.Close()
}

func TestHandleMultiLinkSearch_NoValidResult(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"a", "b"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return httptest.NewRequest(http.MethodGet, "http://x/"+nid, nil), nil
		},
		ResultTester: func(raw []byte) (bool, error) { return false, nil },
	}
	invoker := func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("x")),
		}, nil
	}
	_, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoValidResult)
}
