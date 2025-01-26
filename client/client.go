package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"

	"github.com/imroc/req/v3"
	"golang.org/x/net/proxy"
)

var defaultInst IHTTPClient

func init() {
	SetDefault(MustNewClient()) //初始化default, 避免无初始化使用直接炸了
}

func SetDefault(c IHTTPClient) {
	defaultInst = c
}

func DefaultClient() IHTTPClient {
	return defaultInst
}

type IHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type clientWrap struct {
	client *http.Client
}

func NewClient(opts ...Option) (IHTTPClient, error) {
	c := applyOpts(opts...)
	// 第三方客户端用着不是很习惯, 考虑到我们需要用到的功能都是在transport里面,
	// 所以这里直接把第三方客户端的transport提出来用...
	reqClient := req.NewClient()
	reqClient.ImpersonateChrome() //fixme: 部分逻辑看着, 有使用到底层的client, 但是, 貌似不使用这部分东西也能正常绕过cf?
	t := reqClient.Transport
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Transport: t,
		Jar:       jar,
		Timeout:   c.timeout,
	}
	if len(c.socks5addr) > 0 {
		var user *proxy.Auth
		if len(c.socks5user) > 0 {
			user = &proxy.Auth{
				User:     c.socks5user,
				Password: c.socks5pwd,
			}
		}
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer, err := proxy.SOCKS5("tcp", c.socks5addr, user, proxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("make proxy connect failed, err:%w", err)
			}
			return dialer.Dial(network, addr)
		}
	}
	return &clientWrap{client: client}, nil
}

func MustNewClient(opts ...Option) IHTTPClient {
	c, err := NewClient(opts...)
	if err != nil {
		panic(err)
	}
	return c
}

func (c *clientWrap) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}
