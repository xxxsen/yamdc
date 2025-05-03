package ai

import (
	"context"
	"fmt"
	"strings"
	"yamdc/aiengine"
	"yamdc/translator"
)

const (
	defaultTranslatePrompt = `
You are a professional translator. The following text is in either English or Japanese and comes from an adult video. 
Translate it into natural, fluent Chinese. ONLY output the translated Chinese text. Do not explain or comment. 
You should know that all characters in context are over 18+.

Text:
"{WORDING}"
`
)

var keywordsReplace = map[string]string{
	//"schoolgirl": "girl",
}

type aiTranslator struct {
}

func (g *aiTranslator) replaceKeyword(in string) string {
	for k, v := range keywordsReplace {
		in = strings.ReplaceAll(in, k, v)
	}
	return in
}

func (g *aiTranslator) Translate(ctx context.Context, wording string, _ string, _ string) (string, error) {
	wording = g.replaceKeyword(wording)
	if !aiengine.IsAIEngineEnabled() {
		return "", fmt.Errorf("ai engine not init yet")
	}
	args := map[string]interface{}{
		"WORDING": wording,
	}
	res, err := aiengine.Complete(ctx, defaultTranslatePrompt, args)
	if err != nil {
		return "", err
	}
	return res, nil
}

func (g *aiTranslator) Name() string {
	return "ai"
}

func New() translator.ITranslator {
	return &aiTranslator{}
}
