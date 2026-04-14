package browser

import (
	"context"
	"time"
)

type Params struct {
	WaitSelector string
	WaitTimeout  time.Duration
}

type ctxKey struct{}

func WithParams(ctx context.Context, params *Params) context.Context {
	return context.WithValue(ctx, ctxKey{}, params)
}

func GetParams(ctx context.Context) *Params {
	v, _ := ctx.Value(ctxKey{}).(*Params)
	return v
}
