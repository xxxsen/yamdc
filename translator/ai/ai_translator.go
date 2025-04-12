package ai

import (
	"context"
	"fmt"
	"yamdc/aiengine"
	"yamdc/translator"
)

const (
	defaultTranslatePrompt = `
现在你作为一个资深翻译员, 来帮我翻译文本, 我会给你到一个元组(x, y, z), x, y分别代表原始语言和目标语言, 如果x为空或者auto, 则代表自动识别, z代表需要翻译的文本, 你只需要返回翻译后的文本即可, 不需要任何其他的内容, 也不要有多余的解释, 如果无法翻译则返回空文本。
下面为要翻译的内容: 
("{SRCLANG}", "{DSTLANG}", "{WORDING}")
`
)

type aiTranslator struct {
}

func (g *aiTranslator) Translate(ctx context.Context, wording string, srclang string, dstlang string) (string, error) {
	if !aiengine.IsAIEngineEnabled() {
		return "", fmt.Errorf("ai engine not init yet")
	}
	args := map[string]interface{}{
		"SRCLANG": srclang,
		"DSTLANG": dstlang,
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
