package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/xxxsen/common/logutil"
	basebundle "github.com/xxxsen/yamdc/internal/bundle"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"go.uber.org/zap"
)

func BuildMovieIDCleaner(
	ctx context.Context,
	cli client.IHTTPClient,
	dataDir, sourceType, location string,
) (movieidcleaner.Cleaner, *movieidcleaner.Manager, error) {
	if !HasMovieIDRulesetSource(location) {
		LogMovieIDRulesetConfigMissing(ctx)
		return movieidcleaner.NewPassthroughCleaner(), nil, nil
	}
	st := strings.ToLower(strings.TrimSpace(sourceType))
	if st == "" {
		st = basebundle.SourceTypeLocal
	}
	loc := strings.TrimSpace(location)
	if st == basebundle.SourceTypeLocal {
		resolved, err := ResolveRuleSourcePath(dataDir, loc)
		if err != nil {
			return nil, nil, err
		}
		loc = resolved
	}
	runtimeCleaner := movieidcleaner.NewRuntimeCleaner(nil)
	manager, err := movieidcleaner.NewManager(dataDir, cli, st, loc,
		func(ctx context.Context, rs *movieidcleaner.RuleSet, files []string) error {
			logutil.GetLogger(ctx).Debug("load movieid rules", zap.Strings("files", files))
			inner, innerErr := movieidcleaner.NewCleaner(rs)
			if innerErr != nil {
				return fmt.Errorf("create movieid cleaner: %w", innerErr)
			}
			runtimeCleaner.Swap(inner)
			return nil
		})
	if err != nil {
		return nil, nil, fmt.Errorf("create movieid manager: %w", err)
	}
	if err := manager.Start(ctx); err != nil {
		return nil, nil, fmt.Errorf("start movieid manager: %w", err)
	}
	return runtimeCleaner, manager, nil
}
