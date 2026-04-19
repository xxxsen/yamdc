package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
)

func TestTagPadderHandler(t *testing.T) {
	tests := []struct {
		name       string
		numberID   string
		genres     []string
		wantPrefix string
		wantHas    bool
	}{
		{
			name:       "normal number adds prefix tag",
			numberID:   "ABC-123",
			genres:     []string{"Drama"},
			wantPrefix: "ABC",
			wantHas:    true,
		},
		{
			name:       "pure numeric number no prefix tag",
			numberID:   "123456",
			genres:     []string{"Drama"},
			wantPrefix: "",
			wantHas:    false,
		},
		{
			name:       "underscore separator",
			numberID:   "HEYZO_1234",
			genres:     nil,
			wantPrefix: "HEYZO",
			wantHas:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &tagPadderHandler{}
			num, err := number.Parse(tt.numberID)
			require.NoError(t, err)
			fc := &model.FileContext{
				Number: num,
				Meta:   &model.MovieMeta{Genres: tt.genres},
			}
			err = h.Handle(context.Background(), fc)
			require.NoError(t, err)
			if tt.wantHas {
				assert.Contains(t, fc.Meta.Genres, tt.wantPrefix)
			}
		})
	}
}

func TestGenerateNumberPrefixTag(t *testing.T) {
	tests := []struct {
		name     string
		numberID string
		wantTag  string
		wantOK   bool
	}{
		{
			name:     "letter prefix",
			numberID: "ABC-123",
			wantTag:  "ABC",
			wantOK:   true,
		},
		{
			name:     "pure number",
			numberID: "123456",
			wantTag:  "",
			wantOK:   false,
		},
		{
			name:     "mixed with underscore",
			numberID: "TEST_001",
			wantTag:  "TEST",
			wantOK:   true,
		},
		{
			name:     "single letter",
			numberID: "A-1",
			wantTag:  "A",
			wantOK:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &tagPadderHandler{}
			num, err := number.Parse(tt.numberID)
			require.NoError(t, err)
			fc := &model.FileContext{Number: num}
			tag, ok := h.generateNumberPrefixTag(fc)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantTag, tag)
			}
		})
	}
}

func TestRewriteOrAppendTag(t *testing.T) {
	h := &tagPadderHandler{}

	t.Run("append new tag", func(t *testing.T) {
		meta := &model.MovieMeta{Genres: []string{"existing"}}
		h.rewriteOrAppendTag(meta, "newTag")
		assert.Contains(t, meta.Genres, "newTag")
	})

	t.Run("rewrite existing tag case insensitive", func(t *testing.T) {
		meta := &model.MovieMeta{Genres: []string{"abc"}}
		h.rewriteOrAppendTag(meta, "ABC")
		assert.Contains(t, meta.Genres, "ABC")
		assert.NotContains(t, meta.Genres, "abc")
		assert.Len(t, meta.Genres, 1)
	})

	t.Run("no duplicate when already same case", func(t *testing.T) {
		meta := &model.MovieMeta{Genres: []string{"ABC"}}
		h.rewriteOrAppendTag(meta, "ABC")
		assert.Len(t, meta.Genres, 1)
	})
}
