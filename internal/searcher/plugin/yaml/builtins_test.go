package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectedHost(t *testing.T) {
	assert.Equal(t, "h", selectedHost(&evalContext{host: "h"}, nil))
	assert.Equal(t, "", selectedHost(nil, nil))
	assert.NotEmpty(t, selectedHost(nil, []string{"a"}))
}

func TestResolveMapRef(t *testing.T) {
	v, ok := resolveMapRef("vars.x", "vars.", map[string]string{"x": "y"})
	assert.True(t, ok)
	assert.Equal(t, "y", v)

	_, ok = resolveMapRef("other.x", "vars.", map[string]string{"x": "y"})
	assert.False(t, ok)

	_, ok = resolveMapRef("vars.x", "vars.", nil)
	assert.False(t, ok)
}

// --- condition tests ---

func TestCachedCreator_Error(t *testing.T) {
	cc := &cachedCreator{data: []byte("invalid yaml")}
	_, err := cc.create(nil)
	require.Error(t, err)

	_, err = cc.create(nil)
	require.Error(t, err)
}

func TestCachedCreator_Success(t *testing.T) {
	cc := &cachedCreator{data: []byte(minimalOneStepYAML())}
	plg1, err := cc.create(nil)
	require.NoError(t, err)
	require.NotNil(t, plg1)

	plg2, err := cc.create(nil)
	require.NoError(t, err)
	require.Equal(t, plg1, plg2)
}

// --- currentHost ---

func TestSyncBundle(_ *testing.T) {
	plugins := map[string][]byte{
		"test-plugin": []byte(minimalOneStepYAML()),
	}
	SyncBundle(plugins)
}

func TestBuildRegisterContext(t *testing.T) {
	plugins := map[string][]byte{
		"b-plugin": []byte(minimalOneStepYAML()),
		"a-plugin": []byte(minimalOneStepYAML()),
	}
	ctx := BuildRegisterContext(plugins)
	assert.NotNil(t, ctx)
}

// --- selectedHost ---

func TestSelectedHost_WithContext(t *testing.T) {
	ctx := &evalContext{host: "https://custom.com"}
	assert.Equal(t, "https://custom.com", selectedHost(ctx, []string{"https://default.com"}))
}

func TestSelectedHost_EmptyHosts(t *testing.T) {
	assert.Equal(t, "", selectedHost(nil, nil))
}

// --- OnDecodeHTTPData with postprocess ---
