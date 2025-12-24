package gemini

type config struct {
	Key   string `json:"key"`
	Model string `json:"model"`
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

func applyOpts(opts ...Option) *config {
	c := &config{
		Model: "gemini-2.0-flash",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
