package searcher

import (
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/store"
)

type config struct {
	cli         client.IHTTPClient
	storage     store.IStorage
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

func WithStorage(s store.IStorage) Option {
	return func(c *config) {
		c.storage = s
	}
}

func applyOpts(opts ...Option) *config {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
