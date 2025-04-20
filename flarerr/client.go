package flarerr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"yamdc/client"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

//基于flaresolverr实现

const (
	defaultByPassClientTimeout = 40 * time.Second
)

type ISolverClient interface {
	AddToSolverList(host string) error
	client.IHTTPClient
}

type solverClient struct {
	impl      client.IHTTPClient
	endpoint  string
	timeout   time.Duration
	byPastMap map[string]struct{}
}

func (b *solverClient) convertRequest(oreq *http.Request) (*flareRequest, error) {
	if oreq.Method != http.MethodGet {
		return nil, fmt.Errorf("flare request only support GET method, got %s", oreq.Method)
	}

	req := &flareRequest{
		Cmd:        "request.get",
		Url:        oreq.URL.String(),
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

func (b *solverClient) AddToSolverList(host string) error {
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		uri, err := url.Parse(host)
		if err != nil {
			return err
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
	resp, err := http.Post(b.endpoint+"/v1", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var frResp flareResponse
	if err := json.NewDecoder(resp.Body).Decode(&frResp); err != nil {
		return nil, err
	}
	if frResp.Status != "ok" {
		return nil, fmt.Errorf("flare response status error: %s, message:%s", frResp.Status, frResp.Message)
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

func (b *solverClient) Do(req *http.Request) (*http.Response, error) {
	if b.isNeedByPass(req) {
		logutil.GetLogger(req.Context()).Debug("use solver client for http request to by pass cloudflare protect", zap.String("req", req.URL.String()))
		return b.handleByPassRequest(req)
	}
	return b.impl.Do(req)
}

func NewClient(impl client.IHTTPClient, endpoint string) ISolverClient {
	bc := &solverClient{
		impl:      impl,
		endpoint:  endpoint,
		timeout:   defaultByPassClientTimeout,
		byPastMap: make(map[string]struct{}),
	}
	return bc
}

func MustAddToSolverList(c ISolverClient, host string) {
	if err := c.AddToSolverList(host); err != nil {
		panic(fmt.Sprintf("add host:%s to bypass list failed, err:%v", host, err))
	}
}
