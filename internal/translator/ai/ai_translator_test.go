package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- mock AI engine ----------

type mockEngine struct {
	name        string
	completeRes string
	completeErr error
	capturedP   string
	capturedA   map[string]interface{}
}

func (m *mockEngine) Name() string { return m.name }

func (m *mockEngine) Complete(_ context.Context, prompt string, args map[string]interface{}) (string, error) {
	m.capturedP = prompt
	m.capturedA = args
	return m.completeRes, m.completeErr
}

// ---------- New ----------

func TestNew_DefaultPrompt(t *testing.T) {
	eng := &mockEngine{name: "test"}
	tr := New(eng)
	assert.Equal(t, "ai", tr.Name())

	inner := tr.(*aiTranslator)
	assert.Equal(t, defaultTranslatePrompt, inner.c.prompt)
}

func TestNew_CustomPrompt(t *testing.T) {
	eng := &mockEngine{name: "test"}
	tr := New(eng, WithPrompt("custom {WORDING}"))
	inner := tr.(*aiTranslator)
	assert.Equal(t, "custom {WORDING}", inner.c.prompt)
}

// ---------- Name ----------

func TestAITranslator_Name(t *testing.T) {
	tr := New(&mockEngine{name: "test"})
	assert.Equal(t, "ai", tr.Name())
}

// ---------- Translate ----------

func TestTranslate_Success(t *testing.T) {
	eng := &mockEngine{
		name:        "test",
		completeRes: "翻译结果",
	}
	tr := New(eng, WithPrompt("translate: {WORDING}"))
	result, err := tr.Translate(context.Background(), "hello world", "en", "zh")
	require.NoError(t, err)
	assert.Equal(t, "翻译结果", result)
	assert.Equal(t, "hello world", eng.capturedA["WORDING"])
}

func TestTranslate_EngineError(t *testing.T) {
	eng := &mockEngine{
		name:        "test",
		completeErr: errors.New("api error"),
	}
	tr := New(eng)
	_, err := tr.Translate(context.Background(), "hello", "en", "zh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ai translate failed")
}

func TestTranslate_NilEngine(t *testing.T) {
	tr := &aiTranslator{c: &config{prompt: "p"}, engine: nil}
	_, err := tr.Translate(context.Background(), "hello", "en", "zh")
	require.Error(t, err)
	assert.ErrorIs(t, err, errAIEngineNotInit)
}

func TestTranslate_KeywordReplace(t *testing.T) {
	eng := &mockEngine{
		name:        "test",
		completeRes: "ok",
	}
	tr := New(eng, WithPrompt("{WORDING}"))

	for k, v := range keywordsReplace {
		eng.capturedA = nil
		_, err := tr.Translate(context.Background(), "prefix "+k+" suffix", "en", "zh")
		require.NoError(t, err)
		wording := eng.capturedA["WORDING"].(string)
		assert.NotContains(t, wording, k)
		if v != "" {
			assert.Contains(t, wording, v)
		}
	}
}

func TestTranslate_EmptyWording(t *testing.T) {
	eng := &mockEngine{
		name:        "test",
		completeRes: "",
	}
	tr := New(eng)
	result, err := tr.Translate(context.Background(), "", "en", "zh")
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ---------- WithPrompt ----------

func TestWithPrompt(t *testing.T) {
	c := &config{}
	WithPrompt("my prompt")(c)
	assert.Equal(t, "my prompt", c.prompt)
}

// ---------- replaceKeyword edge cases ----------

func TestReplaceKeyword_NoMatch(t *testing.T) {
	tr := &aiTranslator{c: &config{}}
	result := tr.replaceKeyword("nothing to replace")
	assert.Equal(t, "nothing to replace", result)
}
