package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/xxxsen/yamdc/internal/repository"
)

func OpenAppDB(ctx context.Context, dataDir string) (*repository.SQLite, error) {
	db, err := repository.NewSQLite(ctx, filepath.Join(dataDir, "app", "app.db"))
	if err != nil {
		return nil, fmt.Errorf("init app db failed, err:%w", err)
	}
	return db, nil
}
