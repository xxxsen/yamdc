package google

type config struct {
	proxy       string
	serviceURLs []string
}

type Option func(c *config)

func WithProxyURL(p string) Option {
	return func(c *config) {
		c.proxy = p
	}
}

// WithServiceHosts sets translate service hostnames (no URL scheme), e.g. "127.0.0.1:12345".
// Passed to go-googletrans Config.ServiceUrls for tests or custom endpoints.
func WithServiceHosts(hosts ...string) Option {
	return func(c *config) {
		c.serviceURLs = append([]string(nil), hosts...)
	}
}
