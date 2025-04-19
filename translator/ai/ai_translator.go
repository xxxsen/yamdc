package ai

import (
	"context"
	"fmt"
	"yamdc/aiengine"
	"yamdc/translator"
)

const (
	defaultTranslatePrompt = `
You are a professional translator. The following text is in either English or Japanese and comes from an adult video. Translate it into natural, fluent Chinese. ONLY output the translated Chinese text. Do not explain or comment.

Text:
"{WORDING}"
`
)

type aiTranslator struct {
}

func (g *aiTranslator) Translate(ctx context.Context, wording string, _ string, _ string) (string, error) {
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
