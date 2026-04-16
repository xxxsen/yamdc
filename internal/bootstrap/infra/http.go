package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/flarerr"
	"go.uber.org/zap"
)

type HTTPClientConfig struct {
	TimeoutSec int64
	Proxy      string
}

type FlareSolverrConfig struct {
	Host    string
	Domains map[string]bool
}

func BuildHTTPClient(
	ctx context.Context,
	cfg HTTPClientConfig,
	flareCfg *FlareSolverrConfig,
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
	if flareCfg == nil {
		return clientImpl, nil
	}
	bpc, err := flarerr.New(clientImpl, flareCfg.Host)
	if err != nil {
		return nil, fmt.Errorf("create flaresolverr client failed, err:%w", err)
	}
	domainList := make([]string, 0, len(flareCfg.Domains))
	for domain, ok := range flareCfg.Domains {
		if !ok {
			continue
		}
		domainList = append(domainList, domain)
		logutil.GetLogger(ctx).Debug("add domain to flaresolverr", zap.String("domain", domain))
	}
	flarerr.MustAddToSolverList(bpc, domainList...)
	logutil.GetLogger(ctx).Info("enable flaresolverr client")
	return bpc, nil
}
