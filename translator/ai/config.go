package ai

type config struct {
	prompt string
}

type Option func(c *config)

// WithPrompt 使用{WORDING}作爲占位符
func WithPrompt(pp string) Option {
	return func(c *config) {
		c.prompt = pp
	}
}
