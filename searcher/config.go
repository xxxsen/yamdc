package searcher

import "yamdc/client"

type config struct {
	cli client.IHTTPClient
}

type Option func(c *config)

func WithHTTPClient(cli client.IHTTPClient) Option {
	return func(c *config) {
		c.cli = cli
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
