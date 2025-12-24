package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/client"

	"github.com/xxxsen/common/utils"
)

const (
	defaultOllamaEngineName = "ollama"
)

type ollamaEngine struct {
	c *config
}

func (g *ollamaEngine) Name() string {
	return defaultOllamaEngineName
}

func (g *ollamaEngine) Complete(ctx context.Context, prompt string, args map[string]interface{}) (string, error) {
	bodyReq := buildRequest(prompt, args, g.c.Model)
	raw, err := json.Marshal(bodyReq)
	if err != nil {
		return "", err
	}
	host := strings.TrimRight(g.c.Host, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/generate", host), bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	rsp, err := client.DefaultClient().Do(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama response err, code:%d", rsp.StatusCode)
	}
	var res Response
	if err := json.NewDecoder(rsp.Body).Decode(&res); err != nil {
		return "", err
	}
	if len(res.Error) > 0 {
		return "", fmt.Errorf("ollama response err:%s", res.Error)
	}
	content := strings.TrimSpace(res.Response)
	if len(content) == 0 {
		return "", fmt.Errorf("no result returned from ollama")
	}
	return content, nil
}

func New(opts ...Option) (aiengine.IAIEngine, error) {
	c := applyOpts(opts...)
	return newOllamaEngine(c)
}

func newOllamaEngine(c *config) (*ollamaEngine, error) {
	if len(c.Host) == 0 {
		return nil, fmt.Errorf("host is empty")
	}
	if len(c.Model) == 0 {
		return nil, fmt.Errorf("model is empty")
	}
	return &ollamaEngine{c: c}, nil
}

func createOllamaEngine(args interface{}) (aiengine.IAIEngine, error) {
	c := &config{}
	if err := utils.ConvStructJson(args, c); err != nil {
		return nil, err
	}
	return newOllamaEngine(c)
}

func init() {
	aiengine.Register(defaultOllamaEngineName, createOllamaEngine)
}
