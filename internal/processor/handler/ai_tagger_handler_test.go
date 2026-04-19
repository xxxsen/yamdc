package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
)

type mockAIEngine struct {
	result string
	err    error
}

func (m *mockAIEngine) Name() string { return "mock_ai" }
func (m *mockAIEngine) Complete(_ context.Context, _ string, _ map[string]any) (string, error) {
	return m.result, m.err
}

func TestAITaggerHandlerNilEngine(t *testing.T) {
	h := &aiTaggerHandler{}
	fc := &model.FileContext{Meta: &model.MovieMeta{Title: "test"}}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestAITaggerHandlerTitleTooShort(t *testing.T) {
	h := &aiTaggerHandler{engine: &mockAIEngine{result: "tag1,tag2"}}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title: "short",
			Plot:  "short",
		},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
	assert.Empty(t, fc.Meta.Genres)
}

func TestAITaggerHandlerSuccess(t *testing.T) {
	h := &aiTaggerHandler{engine: &mockAIEngine{result: "标签,测试"}}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title: strings.Repeat("测试标题", 10),
			Plot:  strings.Repeat("这是一个测试情节描述", 20),
		},
	}
	err := h.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Contains(t, fc.Meta.Genres, "AI-标签")
	assert.Contains(t, fc.Meta.Genres, "AI-测试")
}

func TestAITaggerHandlerUsesTranslatedTitle(t *testing.T) {
	h := &aiTaggerHandler{engine: &mockAIEngine{result: "tag1"}}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title:           strings.Repeat("原标题", 10),
			TitleTranslated: strings.Repeat("翻译标题", 10),
			Plot:            strings.Repeat("情节描述", 20),
		},
	}
	err := h.Handle(context.Background(), fc)
	require.NoError(t, err)
}

func TestAITaggerHandlerUsesTranslatedPlot(t *testing.T) {
	h := &aiTaggerHandler{engine: &mockAIEngine{result: "tag1"}}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title:          strings.Repeat("标题内容", 10),
			Plot:           strings.Repeat("情节描述", 20),
			PlotTranslated: strings.Repeat("翻译后的描述", 20),
		},
	}
	err := h.Handle(context.Background(), fc)
	require.NoError(t, err)
}

func TestAITaggerHandlerEngineError(t *testing.T) {
	h := &aiTaggerHandler{engine: &mockAIEngine{err: errors.New("ai error")}}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title: strings.Repeat("长标题", 20),
			Plot:  strings.Repeat("长描述", 40),
		},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
}

func TestAITaggerHandlerFiltersLongTags(t *testing.T) {
	h := &aiTaggerHandler{engine: &mockAIEngine{result: "短标,这是一个非常长的标签"}}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title: strings.Repeat("标题", 15),
			Plot:  strings.Repeat("描述", 40),
		},
	}
	err := h.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Contains(t, fc.Meta.Genres, "AI-短标")
	found := false
	for _, g := range fc.Meta.Genres {
		if strings.Contains(g, "非常长") {
			found = true
		}
	}
	assert.False(t, found, "long tag should be filtered")
}
