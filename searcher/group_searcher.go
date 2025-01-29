package searcher

import (
	"context"
	"yamdc/model"
	"yamdc/number"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type group struct {
	ss []ISearcher
}

func NewGroup(ss []ISearcher) ISearcher {
	return &group{ss: ss}
}
func (g *group) Name() string {
	return "group"
}

func (g *group) Search(ctx context.Context, number *number.Number) (*model.MovieMeta, bool, error) {
	return performGroupSearch(ctx, number, g.ss)
}

func performGroupSearch(ctx context.Context, number *number.Number, ss []ISearcher) (*model.MovieMeta, bool, error) {
	var lastErr error
	for _, s := range ss {
		logutil.GetLogger(ctx).Debug("search number", zap.String("plugin", s.Name()))
		meta, found, err := s.Search(ctx, number)
		if err != nil {
			lastErr = err
			continue
		}
		if !found {
			continue
		}
		return meta, true, nil
	}
	if lastErr != nil {
		return nil, false, lastErr
	}
	return nil, false, nil
}
