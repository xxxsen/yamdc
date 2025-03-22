package google

type config struct {
	proxy string
}

type Option func(c *config)

func WithProxyUrl(p string) Option {
	return func(c *config) {
		c.proxy = p
	}
}
