package translator

import (
	translator "github.com/Conight/go-googletrans"
)

var defaultTranslator *Translator

func Init() error {
	inst, err := New()
	if err != nil {
		return err
	}
	defaultTranslator = inst
	return nil
}

func IsTranslatorEnabled() bool {
	return defaultTranslator != nil
}

type Translator struct {
	t *translator.Translator
}

func New() (*Translator, error) {
	t := translator.New()
	return &Translator{
		t: t,
	}, nil
}

func (t *Translator) Translate(origin, src, dst string) (string, error) {
	res, err := t.t.Translate(origin, src, dst)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

func Translate(origin, src, dst string) (string, error) {
	return defaultTranslator.Translate(origin, src, dst)
}
