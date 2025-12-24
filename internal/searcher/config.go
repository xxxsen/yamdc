package searcher

import "github.com/xxxsen/yamdc/internal/client"

type config struct {
	cli         client.IHTTPClient
	searchCache bool
}

type Option func(c *config)

func WithHTTPClient(cli client.IHTTPClient) Option {
	return func(c *config) {
		c.cli = cli
	}
}

func WithSearchCache(v bool) Option {
	return func(c *config) {
		c.searchCache = v
	}
}

func applyOpts(opts ...Option) *config {
	c := &config{
		cli: client.DefaultClient(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
