package gemini

import (
	"context"
	"encoding/json"
	"os"
	"testing"
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
		os.Setenv(k, v)
	}
}

func TestTranslator(t *testing.T) {
	tt, err := New(WithKey(os.Getenv("GEMINI_KEY")), WithModel("gemini-2.0-flash"), WithClient(client.DefaultClient()))
	assert.NoError(t, err)
	res, err := tt.Translate(context.Background(), "hello world", "", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
	res, err = tt.Translate(context.Background(), "これはテストです", "", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
}
