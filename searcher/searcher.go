package searcher

import (
	"context"
	"yamdc/model"
	"yamdc/number"
)

type ISearcher interface {
	Name() string
	Search(ctx context.Context, number *number.Number) (*model.MovieMeta, bool, error)
}
