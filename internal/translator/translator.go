package translator

import (
	"context"
)

type ITranslator interface {
	Name() string
	Translate(ctx context.Context, wording string, srclang, dstlang string) (string, error)
}

func SetTranslator(t ITranslator) {
	defaultTranslator = t
}

var defaultTranslator ITranslator

func IsTranslatorEnabled() bool {
	return defaultTranslator != nil
}

func Translate(ctx context.Context, origin, src, dst string) (string, error) {
	return defaultTranslator.Translate(ctx, origin, src, dst)
}
