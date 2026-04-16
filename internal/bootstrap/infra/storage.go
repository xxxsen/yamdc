package infra

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/xxxsen/yamdc/internal/store"
)

func BuildCacheStore(ctx context.Context, dataDir string) (store.IStorage, error) {
	s, err := store.NewSqliteStorage(ctx, filepath.Join(dataDir, "cache", "cache.db"))
	if err != nil {
		return nil, fmt.Errorf("init cache store failed: %w", err)
	}
	return s, nil
}
