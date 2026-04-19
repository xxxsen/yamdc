package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/translator"
)

const (
	defaultTranslatePrompt = `
You are a professional translator. The following text is in either English or Japanese
and comes from a movie or entertainment media.
Translate it into natural, fluent Chinese. ONLY output the translated Chinese text.
Do not explain or comment.

Text:
"{WORDING}"
`
)

var errAIEngineNotInit = errors.New("ai engine not init yet")

var keywordsReplace = map[string]string{
	// "schoolgirl": "girl",
}

type aiTranslator struct {
	c      *config
	engine aiengine.IAIEngine
}

func (g *aiTranslator) replaceKeyword(in string) string {
	for k, v := range keywordsReplace {
		in = strings.ReplaceAll(in, k, v)
	}
	return in
}

func (g *aiTranslator) Translate(ctx context.Context, wording, _, _ string) (string, error) {
	wording = g.replaceKeyword(wording)
	if g.engine == nil {
		return "", errAIEngineNotInit
	}
	args := map[string]any{
		"WORDING": wording,
	}
	res, err := g.engine.Complete(ctx, g.c.prompt, args)
	if err != nil {
		return "", fmt.Errorf("ai translate failed: %w", err)
	}
	return res, nil
}

func (g *aiTranslator) Name() string {
	return "ai"
}

func New(engine aiengine.IAIEngine, opts ...Option) translator.ITranslator {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	if len(c.prompt) == 0 {
		c.prompt = defaultTranslatePrompt
	}
	return &aiTranslator{
		c:      c,
		engine: engine,
	}
}
