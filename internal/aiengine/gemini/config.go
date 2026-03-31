package gemini

import "github.com/xxxsen/yamdc/internal/client"

type config struct {
	Key        string             `json:"key"`
	Model      string             `json:"model"`
	HTTPClient client.IHTTPClient `json:"-"`
}

type Option func(*config)

func WithKey(key string) Option {
	return func(c *config) {
		c.Key = key
	}
}

func WithModel(model string) Option {
	return func(c *config) {
		c.Model = model
	}
}

func WithHTTPClient(cli client.IHTTPClient) Option {
	return func(c *config) {
		c.HTTPClient = cli
	}
}

func applyOpts(opts ...Option) *config {
	c := &config{
		Model: "gemini-2.0-flash",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
