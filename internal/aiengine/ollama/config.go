package ollama

import "github.com/xxxsen/yamdc/internal/client"

type config struct {
	Host       string             `json:"host"`
	Model      string             `json:"model"`
	HTTPClient client.IHTTPClient `json:"-"`
}

type Option func(*config)

func WithHost(host string) Option {
	return func(c *config) {
		c.Host = host
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
		Host: "http://127.0.0.1:11434",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
