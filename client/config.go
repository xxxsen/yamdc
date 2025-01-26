package client

import "time"

type config struct {
	timeout time.Duration
	proxy   string
}

type Option func(c *config)

func WithTimeout(t time.Duration) Option {
	return func(c *config) {
		c.timeout = t
	}
}

func WithProxy(link string) Option {
	return func(c *config) {
		c.proxy = link
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
