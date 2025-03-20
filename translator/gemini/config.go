package gemini

import "yamdc/client"

type config struct {
	c     client.IHTTPClient
	key   string
	model string
}

type Option func(*config)

func WithClient(cli client.IHTTPClient) Option {
	return func(c *config) {
		c.c = cli
	}
}

func WithKey(key string) Option {
	return func(c *config) {
		c.key = key
	}
}

func WithModel(model string) Option {
	return func(c *config) {
		c.model = model
	}
}

func applyOpts(opts ...Option) *config {
	c := &config{
		model: "gemini-2.0-flash",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
