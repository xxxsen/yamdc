package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xxxsen/yamdc/internal/bootstrap"
	"github.com/xxxsen/yamdc/internal/config"
	"go.uber.org/zap"
)

func newServerCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start YAMDC HTTP server",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := config.LoadAppConfig(configPath, config.ModeServer)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return runServer(c)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./config.json", "config file")
	return cmd
}

func runServer(c *config.Config) error {
	sc := bootstrap.NewStartContext(c)
	defer func() {
		if err := sc.RunCleanup(context.Background()); err != nil && sc.Infra.Logger != nil {
			sc.Infra.Logger.Error("cleanup start context failed", zap.Error(err))
		}
	}()
	if err := bootstrap.Execute(context.Background(), sc, bootstrap.NewServerActions()); err != nil {
		return fmt.Errorf("server bootstrap failed: %w", err)
	}
	return nil
}
