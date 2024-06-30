package searcher

import (
	"av-capture/model"
	"context"
	"fmt"

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

func (g *group) Search(number string) (*model.AvMeta, error) {
	var lastErr error
	for _, s := range g.ss {
		meta, err := s.Search(number)
		if err != nil {
			logutil.GetLogger(context.Background()).Error("search fail", zap.String("searcher", s.Name()), zap.Error(err))
			lastErr = err
			continue
		}
		return meta, nil
	}
	return nil, fmt.Errorf("not data found, last err:%w", lastErr)
}
