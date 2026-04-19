package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

func restoreRegistry(t *testing.T, saved map[string]CreatorFunc) {
	t.Helper()
	rc := NewRegisterContext()
	for k, v := range saved {
		rc.Register(k, v)
	}
	Swap(rc)
}

func TestRegisterContext_RegisterAndSnapshot(t *testing.T) {
	rc := NewRegisterContext()
	fn := func(_ any) (api.IPlugin, error) { return &api.DefaultPlugin{}, nil }
	rc.Register("alpha", fn)
	rc.Register("beta", fn)

	snap := rc.Snapshot()
	require.Len(t, snap, 2)
	assert.NotNil(t, snap["alpha"])
	assert.NotNil(t, snap["beta"])

	// mutating rc after Snapshot must not affect prior map
	rc.Register("gamma", fn)
	assert.Len(t, snap, 2)
	assert.Len(t, rc.Snapshot(), 3)
}

func TestSwapCreatePluginLookupPlugins(t *testing.T) {
	saved := Snapshot()
	t.Cleanup(func() { restoreRegistry(t, saved) })

	rc := NewRegisterContext()
	rc.Register("factory_unit", func(args any) (api.IPlugin, error) {
		s, _ := args.(string)
		if s == "err" {
			return nil, assert.AnError
		}
		return &api.DefaultPlugin{}, nil
	})
	Swap(rc)

	fn, ok := Lookup("factory_unit")
	require.True(t, ok)
	plg, err := fn("ok")
	require.NoError(t, err)
	require.NotNil(t, plg)

	_, ok = Lookup("missing")
	assert.False(t, ok)

	plg2, err := CreatePlugin("factory_unit", "ok")
	require.NoError(t, err)
	require.NotNil(t, plg2)

	_, err = CreatePlugin("factory_unit", "err")
	assert.Error(t, err)

	_, err = CreatePlugin("nope", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, errFactoryPluginNotFound)

	names := Plugins()
	assert.Equal(t, []string{"factory_unit"}, names)
}

func TestPluginToCreator(t *testing.T) {
	saved := Snapshot()
	t.Cleanup(func() { restoreRegistry(t, saved) })

	want := &api.DefaultPlugin{}
	rc := NewRegisterContext()
	rc.Register("wrap", PluginToCreator(want))
	Swap(rc)

	got, err := CreatePlugin("wrap", nil)
	require.NoError(t, err)
	assert.Same(t, want, got)
}

func TestGlobalSnapshot(t *testing.T) {
	saved := Snapshot()
	t.Cleanup(func() { restoreRegistry(t, saved) })

	rc := NewRegisterContext()
	rc.Register("snap", PluginToCreator(&api.DefaultPlugin{}))
	Swap(rc)

	g := Snapshot()
	require.Contains(t, g, "snap")
	_, err := g["snap"](nil)
	require.NoError(t, err)
}
