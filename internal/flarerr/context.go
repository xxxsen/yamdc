package flarerr

import "context"

// Params marks a request as needing FlareSolverr bypass.
// Its presence in the context is the signal; fields may be added later.
type Params struct{}

type ctxKey struct{}

func WithParams(ctx context.Context, params *Params) context.Context {
	return context.WithValue(ctx, ctxKey{}, params)
}

func GetParams(ctx context.Context) *Params {
	v, _ := ctx.Value(ctxKey{}).(*Params)
	return v
}
