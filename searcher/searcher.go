package searcher

import (
	"av-capture/model"
	"context"
)

type ISearcher interface {
	Name() string
	Search(ctx context.Context, number string) (*model.AvMeta, bool, error)
}
