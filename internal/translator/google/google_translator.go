package google

import (
	"context"
	"github.com/xxxsen/yamdc/internal/translator"

	gt "github.com/Conight/go-googletrans"
)

type googleTranslator struct {
	t *gt.Translator
}

func New(opts ...Option) translator.ITranslator {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	t := gt.New(gt.Config{
		Proxy: c.proxy,
	})
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
		return "", err
	}
	return res.Text, nil
}
