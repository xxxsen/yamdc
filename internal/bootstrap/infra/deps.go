package infra

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/dependency"
)

type DependencySpec struct {
	URL     string
	RelPath string
	Refresh bool
}

func InitDependencies(ctx context.Context, cli client.IHTTPClient, datadir string, specs []DependencySpec) error {
	deps := make([]*dependency.Dependency, 0, len(specs))
	for _, item := range specs {
		deps = append(deps, &dependency.Dependency{
			URL:     item.URL,
			Target:  filepath.Join(datadir, item.RelPath),
			Refresh: item.Refresh,
		})
	}
	if err := dependency.Resolve(ctx, cli, deps); err != nil {
		return fmt.Errorf("resolve dependencies failed: %w", err)
	}
	return nil
}
