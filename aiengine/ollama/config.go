package ollama

type config struct {
	Host  string `json:"host"`
	Model string `json:"model"`
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

func applyOpts(opts ...Option) *config {
	c := &config{
		Host: "http://127.0.0.1:11434",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
