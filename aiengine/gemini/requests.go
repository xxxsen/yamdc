package gemini

import "github.com/xxxsen/common/replacer"

type Request struct {
	Contents         []Content        `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}

type GenerationConfig struct {
	Temperature float64 `json:"temperature"`
	TopK        int     `json:"topK"`
	TopP        float64 `json:"topP"`
}

type PromptFeedback struct {
	BlockReason string `json:"blockReason"`
}

type Response struct {
	PromptFeedback PromptFeedback `json:"promptFeedback"`
	Candidates     []Candidate    `json:"candidates"`
	UsageMetadata  UsageMetadata  `json:"usageMetadata"`
	ModelVersion   string         `json:"modelVersion"`
}

type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason"`
	AvgLogprobs  float64 `json:"avgLogprobs"`
}

type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role"`
}

type Part struct {
	Text string `json:"text"`
}

type UsageMetadata struct {
	PromptTokenCount        int           `json:"promptTokenCount"`
	CandidatesTokenCount    int           `json:"candidatesTokenCount"`
	TotalTokenCount         int           `json:"totalTokenCount"`
	PromptTokensDetails     []TokenDetail `json:"promptTokensDetails"`
	CandidatesTokensDetails []TokenDetail `json:"candidatesTokensDetails"`
}

type TokenDetail struct {
	Modality   string `json:"modality"`
	TokenCount int    `json:"tokenCount"`
}

func buildRequest(prompt string, m map[string]interface{}) *Request {
	res := replacer.ReplaceByMap(prompt, m)
	content := Content{
		Parts: []Part{
			{
				Text: res,
			},
		},
	}
	return &Request{
		Contents: []Content{content},
		GenerationConfig: GenerationConfig{
			Temperature: 0,
			TopK:        1,
			TopP:        0.1,
		},
	}
}
