package browser

import (
	"context"
	"net/http"
	"time"
)

type Params struct {
	WaitSelector       string
	WaitTimeout        time.Duration
	WaitStableDuration time.Duration
	Cookies            []*http.Cookie
	Headers            http.Header
}

type ctxKey struct{}

func WithParams(ctx context.Context, params *Params) context.Context {
	return context.WithValue(ctx, ctxKey{}, params)
}

func GetParams(ctx context.Context) *Params {
	v, _ := ctx.Value(ctxKey{}).(*Params)
	return v
}
