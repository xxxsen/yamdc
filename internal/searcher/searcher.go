package searcher

import (
	"context"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
)

type ISearcher interface {
	Name() string
	Search(ctx context.Context, number *number.Number) (*model.MovieMeta, bool, error)
	Check(ctx context.Context) error
}
