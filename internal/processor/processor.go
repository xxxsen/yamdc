package processor

import (
	"context"
	"github.com/xxxsen/yamdc/internal/model"
)

type IProcessor interface {
	Name() string
	Process(ctx context.Context, meta *model.FileContext) error
}
