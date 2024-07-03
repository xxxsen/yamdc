package processor

import (
	"av-capture/model"
	"context"
)

type IProcessor interface {
	Name() string
	Process(ctx context.Context, meta *model.FileContext) error
}
