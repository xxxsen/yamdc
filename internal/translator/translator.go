package translator

import (
	"context"
)

type ITranslator interface {
	Name() string
	Translate(ctx context.Context, wording, srclang, dstlang string) (string, error)
}
