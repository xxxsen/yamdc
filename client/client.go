package client

import (
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/imroc/req/v3"
)

type Client struct {
	client *http.Client
}

func NewClient() *Client {
	// 第三方客户端用着不是很习惯, 考虑到我们需要用到的功能都是在transport里面,
	// 所以这里直接把第三方客户端的transport提出来用...
	reqClient := req.NewClient()
	reqClient.ImpersonateChrome()
	t := reqClient.Transport
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Transport: t,
		Jar:       jar,
		Timeout:   10 * time.Second,
	}
	return &Client{client: client}
}

func (c *Client) GetOriginClient() *http.Client {
	return c.client
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}
