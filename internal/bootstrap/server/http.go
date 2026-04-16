package server

import (
	"fmt"
	"os"

	"github.com/xxxsen/yamdc/internal/web"
	"go.uber.org/zap"
)

func ServeHTTP(api *web.API, logger *zap.Logger, scanDir, dataDir string) error {
	addr := os.Getenv("YAMDC_SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if logger != nil {
		logger.Info("yamdc server start",
			zap.String("addr", addr),
			zap.String("scan_dir", scanDir),
			zap.String("data_dir", dataDir),
		)
	}
	engine, err := api.Engine(addr)
	if err != nil {
		return fmt.Errorf("init web engine failed, err:%w", err)
	}
	if err := engine.Run(); err != nil {
		return fmt.Errorf("listen and serve failed, err:%w", err)
	}
	return nil
}
