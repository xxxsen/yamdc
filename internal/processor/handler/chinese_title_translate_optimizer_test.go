package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
)

func TestReadTitleFromCNumber(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	c.m["TEST-999"] = "Test Chinese Title"

	t.Run("found", func(t *testing.T) {
		title, ok, err := c.readTitleFromCNumber(context.Background(), "TEST-999")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "Test Chinese Title", title)
	})

	t.Run("not found", func(t *testing.T) {
		title, ok, err := c.readTitleFromCNumber(context.Background(), "NONEXIST-123")
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Empty(t, title)
	})
}

func TestReadTitleFromCNumberUninitialized(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	c.tryInitCNumber(context.Background())
	_, ok, err := c.readTitleFromCNumber(context.Background(), "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestChineseTitleOptimizeHandleWithCNumber(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	c.m["ABC-123"] = "Chinese Title"

	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Number: "ABC-123"},
	}
	err := c.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Equal(t, "Chinese Title", fc.Meta.TitleTranslated)
}

func TestChineseTitleOptimizeHandleNoResult(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	num, _ := number.Parse("XYZ-999")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Number: "XYZ-999"},
	}
	err := c.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Empty(t, fc.Meta.TitleTranslated)
}

func TestChineseTitleOptimizeHandleSubHandlerError(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	num, _ := number.Parse("GHI-789")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Number: "GHI-789"},
	}
	err := c.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Empty(t, fc.Meta.TitleTranslated)
}
