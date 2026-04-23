package infra

import (
	"context"
	"fmt"

	"github.com/xxxsen/yamdc/internal/store"
)

func BuildCacheStore(ctx context.Context, dataDir string) (store.IStorage, error) {
	s, err := store.NewPebbleStorage(ctx, store.PebblePathForDataDir(dataDir))
	if err != nil {
		return nil, fmt.Errorf("init cache store failed: %w", err)
	}
	return s, nil
}
