package translater

import (
	translator "github.com/Conight/go-googletrans"
)

var defaultTranslater *Translater

func Init() error {
	t := translator.New()
	defaultTranslater = &Translater{
		t: t,
	}
	return nil
}

type Translater struct {
	t *translator.Translator
}

func GetDefault() *Translater {
	return defaultTranslater
}

func (t *Translater) Translate(origin, src, dst string) (string, error) {
	res, err := t.t.Translate(origin, src, dst)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}
