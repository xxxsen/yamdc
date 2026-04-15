package flarerr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/client"
	"go.uber.org/zap"
)

var (
	errFlareOnlyGET        = errors.New("flare request only supports GET method")
	errFlareResponseStatus = errors.New("flare response status error")
)

// 基于flaresolverr实现
const (
	defaultByPassClientTimeout = 40 * time.Second
)

type ICloudflareSolverClient interface {
	AddHost(host string) error
	client.IHTTPClient
}

type solverClient struct {
	impl      client.IHTTPClient
	endpoint  string
	timeout   time.Duration
	byPastMap map[string]struct{}
	tested    bool
}

func (b *solverClient) convertRequest(oreq *http.Request) (*flareRequest, error) {
	if oreq.Method != http.MethodGet {
		return nil, fmt.Errorf("%w, got %s", errFlareOnlyGET, oreq.Method)
	}

	req := &flareRequest{
		Cmd:        "request.get",
		URL:        oreq.URL.String(),
		MaxTimeout: int(b.timeout.Milliseconds()),
	}

	return req, nil
}

func (b *solverClient) isNeedByPass(req *http.Request) bool {
	if _, ok := b.byPastMap[req.Host]; ok {
		return true
	}
	return false
}

func (b *solverClient) AddHost(host string) error {
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		uri, err := url.Parse(host)
		if err != nil {
			return fmt.Errorf("parse host url: %w", err)
		}
		host = uri.Host
	}
	b.byPastMap[host] = struct{}{}
	return nil
}

func (b *solverClient) handleByPassRequest(req *http.Request) (*http.Response, error) {
	fr, err := b.convertRequest(req)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(fr)
	//nolint:gosec,noctx // internal solver endpoint with controlled URL
	resp, err := http.Post(b.endpoint+"/v1", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("post to flare solver: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var frResp flareResponse
	if err := json.NewDecoder(resp.Body).Decode(&frResp); err != nil {
		return nil, fmt.Errorf("decode flare response: %w", err)
	}
	if frResp.Status != "ok" {
		return nil, fmt.Errorf("%w: %s, message: %s", errFlareResponseStatus, frResp.Status, frResp.Message)
	}
	// 返回一个伪造的 http.Response
	return &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: frResp.Solution.Status,
		Body:       io.NopCloser(bytes.NewReader([]byte(frResp.Solution.Response))),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func (b *solverClient) testHost(ctx context.Context, impl client.IHTTPClient, endpoint string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	//nolint:gosec // internal solver connectivity test
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create test request: %w", err)
	}
	rsp, err := impl.Do(req)
	if err != nil {
		return fmt.Errorf("execute test request: %w", err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	return nil
}

func (b *solverClient) Do(req *http.Request) (*http.Response, error) {
	if !b.tested { // 测试通过后续就不再测试了
		if err := b.testHost(req.Context(), b.impl, b.endpoint); err != nil {
			return nil, fmt.Errorf("test solver host failed, endpoint:%s, err:%w", b.endpoint, err)
		}
		b.tested = true
	}

	if b.isNeedByPass(req) {
		logutil.GetLogger(req.Context()).Debug("use solver client for http request to by pass cloudflare protect",
			zap.String("req", req.URL.String()),
		)
		return b.handleByPassRequest(req)
	}
	rsp, err := b.impl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("solver passthrough request: %w", err)
	}
	return rsp, nil
}

func New(impl client.IHTTPClient, endpoint string) (ICloudflareSolverClient, error) {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	bc := &solverClient{
		impl:      impl,
		endpoint:  endpoint,
		timeout:   defaultByPassClientTimeout,
		byPastMap: make(map[string]struct{}),
	}
	return bc, nil
}

func MustAddToSolverList(c ICloudflareSolverClient, hosts ...string) {
	for _, host := range hosts {
		if err := c.AddHost(host); err != nil {
			panic(fmt.Sprintf("add host:%s to bypass list failed, err:%v", host, err))
		}
	}
}
