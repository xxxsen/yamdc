package ollama

import "github.com/xxxsen/common/replacer"

type Request struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type Response struct {
	Model      string `json:"model"`
	CreatedAt  string `json:"created_at"`
	Response   string `json:"response"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason"`
	Error      string `json:"error"`
}

func buildRequest(prompt string, args map[string]interface{}, model string) *Request {
	p := replacer.ReplaceByMap(prompt, args)
	return &Request{
		Model:  model,
		Prompt: p,
		Stream: false,
	}
}
