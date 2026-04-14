package browser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/xxxsen/yamdc/internal/client"
)

type INavigator interface {
	Navigate(ctx context.Context, url string, params *Params) ([]byte, error)
	Close() error
}

type httpClientWrap struct {
	impl      client.IHTTPClient
	navigator INavigator
}

func NewHTTPClient(impl client.IHTTPClient, navigator INavigator) client.IHTTPClient {
	return &httpClientWrap{impl: impl, navigator: navigator}
}

func (c *httpClientWrap) Do(req *http.Request) (*http.Response, error) {
	params := GetParams(req.Context())
	if params == nil {
		return c.impl.Do(req)
	}
	body, err := c.navigator.Navigate(req.Context(), req.URL.String(), params)
	if err != nil {
		return nil, fmt.Errorf("browser navigate failed: %w", err)
	}
	return &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}, nil
}
