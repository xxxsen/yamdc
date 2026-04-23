package google

import (
	"context"
	"fmt"

	gt "github.com/Conight/go-googletrans"

	"github.com/xxxsen/yamdc/internal/translator"
)

type googleTranslator struct {
	t *gt.Translator
}

func New(opts ...Option) translator.ITranslator {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	gcfg := gt.Config{
		Proxy: c.proxy,
	}
	if len(c.serviceURLs) > 0 {
		gcfg.ServiceUrls = c.serviceURLs
	}
	t := gt.New(gcfg)
	return &googleTranslator{
		t: t,
	}
}

func (t *googleTranslator) Name() string {
	return "google"
}

// Translate wraps the underlying go-googletrans call with context support.
// Because go-googletrans does not accept a context, a background goroutine
// is used: if ctx is canceled before the call returns, Translate returns
// immediately with the cancellation error, but the goroutine lives on until
// the underlying HTTP round-trip completes (bounded by the HTTP client timeout).
//
//nolint:contextcheck // defensive nil-ctx fallback
func (t *googleTranslator) Translate(ctx context.Context, wording, src, dst string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		res, err := t.t.Translate(wording, src, dst)
		if err != nil {
			ch <- result{err: fmt.Errorf("google translate failed: %w", err)}
			return
		}
		ch <- result{text: res.Text}
	}()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("google translate canceled: %w", ctx.Err())
	case r := <-ch:
		return r.text, r.err
	}
}
