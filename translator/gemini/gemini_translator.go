package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"yamdc/translator"
)

const (
	prompt = `
现在你作为一个资深翻译员, 来帮我翻译文本, 我会给你到一个元组(x, y, z), x, y分别代表原始语言和目标语言, 如果x为空, 则代表自动识别, z代表需要翻译的文本, 你只需要返回翻译后的文本即可, 不需要任何其他的内容, 也不要有多余的解释, 如果无法翻译则返回空文本。
下面为要翻译的内容: 
("{SRCLANG}", "{DSTLANG}", "{WORDING}")
`
)

type geminiTranslator struct {
	c *config
}

func (g *geminiTranslator) Translate(ctx context.Context, wording string, srclang string, dstlang string) (string, error) {
	m := map[string]interface{}{
		"SRCLANG": srclang,
		"DSTLANG": dstlang,
		"WORDING": wording,
	}
	bodyRes := buildRequest(prompt, m)
	raw, err := json.Marshal(bodyRes)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.c.model, g.c.key), bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rsp, err := g.c.c.Do(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini response err, code:%d", rsp.StatusCode)
	}
	var res Response
	if err := json.NewDecoder(rsp.Body).Decode(&res); err != nil {
		return "", err
	}
	if len(res.Candidates) == 0 {
		return "", fmt.Errorf("no translate result found")
	}
	if len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no translate result part found")
	}
	if len(res.Candidates[0].Content.Parts[0].Text) == 0 {
		return "", fmt.Errorf("no translate result text found")
	}
	return res.Candidates[0].Content.Parts[0].Text, nil
}

func New(opts ...Option) (translator.ITranslator, error) {
	c := applyOpts(opts...)
	if c.c == nil {
		return nil, fmt.Errorf("client is nil")
	}
	if c.key == "" {
		return nil, fmt.Errorf("key is empty")
	}
	if c.model == "" {
		return nil, fmt.Errorf("model is empty")
	}
	return &geminiTranslator{c: c}, nil
}
