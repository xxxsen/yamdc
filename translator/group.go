package translator

import (
	"context"
	"fmt"
)

type group struct {
	impls []ITranslator
}

func (g *group) Name() string {
	names := make([]string, len(g.impls))
	for i, impl := range g.impls {
		names[i] = impl.Name()
	}
	return "group(" + names[0] + ")"
}

func (g *group) Translate(ctx context.Context, wording string, srclang string, dstlang string) (string, error) {
	var retErr error
	for _, impl := range g.impls {
		res, err := impl.Translate(ctx, wording, srclang, dstlang)
		if err != nil {
			retErr = err
			continue
		}
		return res, nil
	}
	return "", fmt.Errorf("unable to translate, last error: %w", retErr)
}

func NewGroup(impls ...ITranslator) ITranslator {
	return &group{
		impls: impls,
	}
}
