package searcher

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/store"
)

type debuggerTestClient struct{}

func (debuggerTestClient) Do(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected http request: %s", req.URL.String())
}

type precheckFalsePlugin struct {
	api.DefaultPlugin
}

func (p *precheckFalsePlugin) OnPrecheckRequest(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func TestDebuggerUsesSnapshotCreators(t *testing.T) {
	oldCtx := factory.NewRegisterContext()
	oldCtx.Register("old", func(_ interface{}) (api.IPlugin, error) {
		return &precheckFalsePlugin{}, nil
	})
	factory.Swap(oldCtx)

	debugger := NewDebugger(debuggerTestClient{}, store.NewMemStorage(), nil, []string{"old"}, nil)

	newCtx := factory.NewRegisterContext()
	newCtx.Register("new", func(_ interface{}) (api.IPlugin, error) {
		return &precheckFalsePlugin{}, nil
	})
	factory.Swap(newCtx)

	plugins := debugger.Plugins()
	require.Equal(t, []string{"old"}, plugins.Available)

	result, err := debugger.DebugSearch(context.Background(), DebugSearchOptions{
		Input:      "ABC-123",
		UseCleaner: false,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"old"}, result.UsedPlugins)
	require.Len(t, result.PluginResults, 1)
	require.Equal(t, "old", result.PluginResults[0].Plugin)
	require.False(t, result.PluginResults[0].Found)
}
