package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/model"
)

func TestDedupPreferUpperBasic(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{"4K 大写胜小写", []string{"4K", "4k"}, []string{"4K"}},
		{"VR 多大写优先", []string{"vr", "Vr", "VR"}, []string{"VR"}},
		{"无冲突原样保留", []string{"4K", "VR", "字幕版"}, []string{"4K", "VR", "字幕版"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, dedupPreferUpper(tt.input))
		})
	}
}

func TestDedupPreferUpperMixedCJK(t *testing.T) {
	assert.Equal(t, []string{"字幕版"}, dedupPreferUpper([]string{"字幕版", "字幕版"}))
	assert.Equal(t,
		[]string{"4K", "字幕版"},
		dedupPreferUpper([]string{"4K", "字幕版", "4k"}),
		"保留首次出现的 lowercase key 顺序")
}

func TestDedupPreferUpperStableTie(t *testing.T) {
	assert.Equal(t,
		[]string{"Ab"},
		dedupPreferUpper([]string{"Ab", "aB"}),
		"平票时保留首次出现的变体")
	assert.Equal(t,
		[]string{"aB"},
		dedupPreferUpper([]string{"aB", "Ab"}),
		"平票时保留首次出现的变体 (顺序反过来)")
}

func TestDedupPreferUpperIdempotent(t *testing.T) {
	input := []string{"4K", "4k", "vr", "VR", "字幕版", "字幕版"}
	first := dedupPreferUpper(input)
	second := dedupPreferUpper(first)
	assert.Equal(t, first, second, "幂等: f(f(x)) == f(x)")
}

func TestDedupPreferUpperEmpty(t *testing.T) {
	assert.Empty(t, dedupPreferUpper(nil))
	assert.Empty(t, dedupPreferUpper([]string{}))
}

func TestDedupPreferUpperNoDuplicate(t *testing.T) {
	input := []string{"4K", "8K", "VR", "字幕版", "特别版"}
	assert.Equal(t, input, dedupPreferUpper(input))
}

func TestCountUpperASCII(t *testing.T) {
	assert.Equal(t, 1, countUpperASCII("4K"))
	assert.Equal(t, 0, countUpperASCII("4k"))
	assert.Equal(t, 2, countUpperASCII("VR"))
	assert.Equal(t, 1, countUpperASCII("Vr"))
	assert.Equal(t, 0, countUpperASCII("字幕版"))
	assert.Equal(t, 0, countUpperASCII(""))
}

func TestTagDedupHandlerNilMeta(t *testing.T) {
	h := &tagDedupHandler{}
	fc := &model.FileContext{Meta: nil}
	err := h.Handle(context.Background(), fc)
	require.NoError(t, err)
}

func TestTagDedupHandlerEmptyGenres(t *testing.T) {
	h := &tagDedupHandler{}
	fc := &model.FileContext{Meta: &model.MovieMeta{Genres: nil}}
	require.NoError(t, h.Handle(context.Background(), fc))
	assert.Nil(t, fc.Meta.Genres)

	fc.Meta.Genres = []string{}
	require.NoError(t, h.Handle(context.Background(), fc))
	assert.Empty(t, fc.Meta.Genres)
}

func TestTagDedupHandlerDedupInPlace(t *testing.T) {
	h := &tagDedupHandler{}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Genres: []string{"4K", "4k", "VR", "vr", "字幕版"},
		},
	}
	require.NoError(t, h.Handle(context.Background(), fc))
	assert.Equal(t, []string{"4K", "VR", "字幕版"}, fc.Meta.Genres)
}

func TestTagDedupHandlerViaFactory(t *testing.T) {
	h, err := CreateHandler(HTagDedup, nil, appdeps.Runtime{})
	require.NoError(t, err)
	assert.NotNil(t, h)
}
