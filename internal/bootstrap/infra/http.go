package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/xxxsen/yamdc/internal/client"
)

type HTTPClientConfig struct {
	TimeoutSec int64
	Proxy      string
}

type FlareSolverrConfig struct {
	Host string
}

func BuildHTTPClient(
	_ context.Context,
	cfg HTTPClientConfig,
) (client.IHTTPClient, error) {
	opts := make([]client.Option, 0, 4)
	if cfg.TimeoutSec > 0 {
		opts = append(opts, client.WithTimeout(time.Duration(cfg.TimeoutSec)*time.Second))
	}
	if cfg.Proxy != "" {
		opts = append(opts, client.WithProxy(cfg.Proxy))
	}
	clientImpl, err := client.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create http client failed: %w", err)
	}
	return clientImpl, nil
}
