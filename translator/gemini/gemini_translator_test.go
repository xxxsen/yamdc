package gemini

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"yamdc/aiengine"
	"yamdc/aiengine/gemini"
	"yamdc/client"

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
	eng, err := gemini.NewGeminiEngine(gemini.WithKey(os.Getenv("GEMINI_KEY")), gemini.WithModel("gemini-2.0-flash"), gemini.WithClient(client.DefaultClient()))
	assert.NoError(t, err)
	aiengine.SetAIEngine(eng)
	assert.NoError(t, err)

	tt, err := New()
	assert.NoError(t, err)

	res, err := tt.Translate(context.Background(), "hello world", "", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
	res, err = tt.Translate(context.Background(), "これはテストです", "", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
}
