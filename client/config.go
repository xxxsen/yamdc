package client

import "time"

type config struct {
	timeout    time.Duration
	socks5addr string
	socks5user string
	socks5pwd  string
}

type Option func(c *config)

func WithTimeout(t time.Duration) Option {
	return func(c *config) {
		c.timeout = t
	}
}

func WithSocks5Proxy(addr string, user string, pwd string) Option {
	return func(c *config) {
		c.socks5addr = addr
		c.socks5user = user
		c.socks5pwd = pwd
	}
}

func applyOpts(opts ...Option) *config {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	if c.timeout == 0 {
		c.timeout = 10 * time.Second
	}
	return c
}
