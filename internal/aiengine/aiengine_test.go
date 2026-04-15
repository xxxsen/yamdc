package aiengine_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/aiengine"
	_ "github.com/xxxsen/yamdc/internal/aiengine/gemini"
	_ "github.com/xxxsen/yamdc/internal/aiengine/ollama"
)

func TestResolveCreateConfig_WithHTTPClient(t *testing.T) {
	cli := http.DefaultClient
	cfg := aiengine.ResolveCreateConfig(aiengine.WithHTTPClient(cli))
	require.Equal(t, cli, cfg.HTTPClient)
}

func TestResolveCreateConfig_Empty(t *testing.T) {
	cfg := aiengine.ResolveCreateConfig()
	require.Nil(t, cfg.HTTPClient)
}

func TestCreate_UnknownEngine(t *testing.T) {
	_, err := aiengine.Create("definitely-not-registered-engine-xyz", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown ai engine")
}

func TestCreate_RegisteredEngine_Gemini(t *testing.T) {
	_, err := aiengine.Create("gemini", map[string]interface{}{
		"key":   "test-key",
		"model": "test-model",
	})
	require.NoError(t, err)
}

func TestRegister_DuplicatePanics(t *testing.T) {
	require.Panics(t, func() {
		aiengine.Register("gemini", func(_ interface{}, _ ...aiengine.CreateOption) (aiengine.IAIEngine, error) {
			return nil, nil
		})
	})
}
