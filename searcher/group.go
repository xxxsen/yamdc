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

func (g *group) Search(ctx context.Context, number *number.Number) (*model.AvMeta, bool, error) {
	var lastErr error
	for _, s := range g.ss {
		meta, found, err := s.Search(ctx, number)
		if err != nil {
			logutil.GetLogger(context.Background()).Error("search fail", zap.String("searcher", s.Name()), zap.Error(err))
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
