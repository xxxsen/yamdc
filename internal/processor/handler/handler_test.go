package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"
)

type noopHandler struct{}

func (h *noopHandler) Handle(_ context.Context, _ *model.FileContext) error { return nil }

func TestRegisterAndCreateHandler(t *testing.T) {
	name := "test_handler_register"
	Register(name, ToCreator(&noopHandler{}))
	h, err := CreateHandler(name, nil, appdeps.Runtime{})
	require.NoError(t, err)
	assert.NotNil(t, h)
}

func TestCreateHandlerNotFound(t *testing.T) {
	h, err := CreateHandler("nonexistent_handler_xyz", nil, appdeps.Runtime{})
	assert.Error(t, err)
	assert.Nil(t, h)
	assert.ErrorIs(t, err, errHandlerNotFound)
}

func TestHandlers(t *testing.T) {
	list := Handlers()
	assert.NotEmpty(t, list)
	seen := make(map[string]bool)
	for _, name := range list {
		assert.False(t, seen[name], "duplicate handler name: %s", name)
		seen[name] = true
	}
	for i := 1; i < len(list); i++ {
		assert.True(t, list[i-1] <= list[i], "Handlers() should be sorted")
	}
}

func TestToCreator(t *testing.T) {
	h := &noopHandler{}
	creator := ToCreator(h)
	result, err := creator(nil, appdeps.Runtime{})
	require.NoError(t, err)
	assert.Equal(t, h, result)
}

func TestCreateAllRegisteredHandlers(t *testing.T) {
	deps := appdeps.Runtime{
		Storage: store.NewMemStorage(),
	}
	handlerNames := []string{
		HPosterCropper,
		HDurationFixer,
		HImageTranscoder,
		HTranslater,
		HWatermakrMaker,
		HTagPadder,
		HNumberTitle,
		HActorSpliter,
		HAITagger,
		HHDCoverHandler,
		HChineseTitleTranslateOptimizer,
	}
	for _, name := range handlerNames {
		t.Run(name, func(t *testing.T) {
			h, err := CreateHandler(name, nil, deps)
			require.NoError(t, err)
			assert.NotNil(t, h)
		})
	}
}

func TestCreateTagMapperHandlerViaFactory(t *testing.T) {
	deps := appdeps.Runtime{
		Storage: store.NewMemStorage(),
	}
	h, err := CreateHandler(HTagMapper, map[string]any{}, deps)
	require.NoError(t, err)
	assert.NotNil(t, h)
}
