package flarerr

import (
	"errors"
	"net/http"
)

type mockHTTPClient struct {
	doFn func(*http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.doFn == nil {
		return nil, errors.New("doFn not set")
	}
	return m.doFn(req)
}
