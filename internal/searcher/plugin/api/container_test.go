package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitContainerAndGetSetExportImport(t *testing.T) {
	ctx := InitContainer(context.Background())

	SetContainerValue(ctx, "k1", "v1")
	v, ok := GetContainerValue(ctx, "k1")
	require.True(t, ok)
	assert.Equal(t, "v1", v)

	_, ok = GetContainerValue(ctx, "missing")
	assert.False(t, ok)

	m := ExportContainerData(ctx)
	assert.Equal(t, map[string]string{"k1": "v1"}, m)

	other := InitContainer(context.Background())
	SetContainerValue(other, "x", "y")
	ImportContainerData(ctx, ExportContainerData(other))
	v, ok = GetContainerValue(ctx, "x")
	require.True(t, ok)
	assert.Equal(t, "y", v)
}

func TestMustGetContainer_PanicsWithoutInit(t *testing.T) {
	assert.Panics(t, func() {
		GetContainerValue(context.Background(), "k")
	})
	assert.Panics(t, func() {
		SetContainerValue(context.Background(), "k", "v")
	})
	assert.Panics(t, func() {
		ExportContainerData(context.Background())
	})
	assert.Panics(t, func() {
		ImportContainerData(context.Background(), map[string]string{"a": "b"})
	})
}

func TestMustGetContainer_PanicsWithWrongContextValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), defaultContainerTypeKey, "not-a-container")
	assert.Panics(t, func() {
		GetContainerValue(ctx, "k")
	})
}
