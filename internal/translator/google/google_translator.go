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

func (t *googleTranslator) Translate(_ context.Context, wording, src, dst string) (string, error) {
	res, err := t.t.Translate(wording, src, dst)
	if err != nil {
		return "", fmt.Errorf("google translate failed: %w", err)
	}
	return res.Text, nil
}
