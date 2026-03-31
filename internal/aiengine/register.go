package aiengine

import "github.com/xxxsen/yamdc/internal/client"

var (
	m = make(map[string]AIEngineCreator)
)

type CreateOption func(*createConfig)

type createConfig struct {
	httpClient client.IHTTPClient
}

type CreateConfig struct {
	HTTPClient client.IHTTPClient
}

func WithHTTPClient(cli client.IHTTPClient) CreateOption {
	return func(c *createConfig) {
		c.httpClient = cli
	}
}

func applyCreateOpts(opts ...CreateOption) *createConfig {
	cc := &createConfig{}
	for _, opt := range opts {
		opt(cc)
	}
	return cc
}

func ResolveCreateConfig(opts ...CreateOption) CreateConfig {
	cc := applyCreateOpts(opts...)
	return CreateConfig{
		HTTPClient: cc.httpClient,
	}
}

type AIEngineCreator func(args interface{}, opts ...CreateOption) (IAIEngine, error)

func Create(name string, args interface{}, opts ...CreateOption) (IAIEngine, error) {
	if creator, ok := m[name]; ok {
		return creator(args, opts...)
	}
	return nil, nil
}
func Register(name string, creator AIEngineCreator) {
	if _, ok := m[name]; ok {
		panic("ai engine already registered")
	}
	m[name] = creator
}
