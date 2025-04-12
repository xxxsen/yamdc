package ai

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"yamdc/aiengine"
	"yamdc/aiengine/gemini"

	"github.com/stretchr/testify/assert"
)

func init() {
	raw, err := os.ReadFile("../../.vscode/keys.json")
	if err != nil {
		panic(err)
	}
	keys := make(map[string]string)
	if err := json.Unmarshal(raw, &keys); err != nil {
		panic(err)
	}
	for k, v := range keys {
		_ = os.Setenv(k, v)
	}
}

func TestTranslator(t *testing.T) {
	eng, err := gemini.New(gemini.WithKey(os.Getenv("GEMINI_KEY")), gemini.WithModel("gemini-2.0-flash"))
	assert.NoError(t, err)
	aiengine.SetAIEngine(eng)
	assert.NoError(t, err)

	tt := New()

	res, err := tt.Translate(context.Background(), "hello world", "", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
	res, err = tt.Translate(context.Background(), "これはテストです", "", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
}
