package searcher

import (
	"av-capture/model"
	"av-capture/number"
	"context"
)

type ISearcher interface {
	Name() string
	Search(ctx context.Context, number *number.Number) (*model.AvMeta, bool, error)
}
