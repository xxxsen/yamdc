package twostep

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

func TestHandleMultiLinkSearch_ReqBuilderError(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"a"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(_ string) (*http.Request, error) {
			return nil, errors.New("build fail")
		},
		ResultTester: func(_ []byte) (bool, error) { return true, nil },
	}
	rsp, err := HandleMultiLinkSearch(context.Background(), nil, xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build request")
}

func TestHandleMultiLinkSearch_InvokerError(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"a"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, "http://x/"+nid, nil)
		},
		ResultTester: func(_ []byte) (bool, error) { return true, nil },
	}
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return nil, errors.New("dial error")
	}
	rsp, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step search failed")
}

func TestHandleMultiLinkSearch_SkipsBadStatus(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"n1", "n2"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, "http://x/"+nid, nil)
		},
		ResultTester: func(raw []byte) (bool, error) {
			return strings.Contains(string(raw), "GOOD"), nil
		},
	}
	calls := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
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
			return http.NewRequestWithContext(context.Background(), http.MethodGet, "http://x/"+nid, nil)
		},
		ResultTester: func(_ []byte) (bool, error) { return true, nil },
	}
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errReader{}),
		}, nil
	}
	rsp, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read data as html")
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read err")
}

func TestHandleMultiLinkSearch_ResultTesterError(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"n1"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, "http://x/"+nid, nil)
		},
		ResultTester: func(_ []byte) (bool, error) {
			return false, errors.New("tester boom")
		},
	}
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("data")),
		}, nil
	}
	rsp, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test result failed")
}

func TestHandleMultiLinkSearch_ResultTesterFalseThenSuccess(t *testing.T) {
	xctx := &MultiLinkContext{
		Numbers:         []string{"n1", "n2"},
		ValidStatusCode: []int{200},
		ReqBuilder: func(nid string) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, "http://x/"+nid, nil)
		},
		ResultTester: func(raw []byte) (bool, error) {
			return strings.Contains(string(raw), "WIN"), nil
		},
	}
	calls := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
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
			return http.NewRequestWithContext(context.Background(), http.MethodGet, "http://x/"+nid, nil)
		},
		ResultTester: func(_ []byte) (bool, error) { return false, nil },
	}
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("x")),
		}, nil
	}
	rsp, err := HandleMultiLinkSearch(context.Background(), api.HTTPInvoker(invoker), xctx)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoValidResult)
}
