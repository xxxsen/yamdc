package searcher

import (
	"context"
	"yamdc/internal/model"
	"yamdc/internal/number"
)

type ISearcher interface {
	Name() string
	Search(ctx context.Context, number *number.Number) (*model.MovieMeta, bool, error)
	Check(ctx context.Context) error
}
