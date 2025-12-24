package processor

import (
	"context"
	"yamdc/internal/model"
)

type IProcessor interface {
	Name() string
	Process(ctx context.Context, meta *model.FileContext) error
}
