package googletranslator

import (
	"context"
	"yamdc/translator"

	gt "github.com/Conight/go-googletrans"
)

type googleTranslator struct {
	t *gt.Translator
}

func New(opts ...Option) (translator.ITranslator, error) {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	t := gt.New(gt.Config{
		Proxy: c.proxy,
	})
	return &googleTranslator{
		t: t,
	}, nil
}

func (t *googleTranslator) Translate(_ context.Context, wording, src, dst string) (string, error) {
	res, err := t.t.Translate(wording, src, dst)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}
