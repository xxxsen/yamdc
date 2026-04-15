package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
)

func TestSplitActor(t *testing.T) {
	tests := []struct {
		name     string
		actors   []string
		expected []string
	}{
		{
			name:     "split parenthesized alias",
			actors:   []string{"永野司 (永野つかさ)"},
			expected: []string{"永野司", "永野つかさ"},
		},
		{
			name:     "split fullwidth parenthesized alias",
			actors:   []string{"萨达（AA萨达）"},
			expected: []string{"萨达", "AA萨达"},
		},
		{
			name:     "no parentheses",
			actors:   []string{"Simple Name"},
			expected: []string{"Simple Name"},
		},
		{
			name:     "empty list",
			actors:   []string{},
			expected: []string{},
		},
		{
			name:     "whitespace trimming",
			actors:   []string{"  Actor A  "},
			expected: []string{"Actor A"},
		},
		{
			name:     "multiple actors mixed",
			actors:   []string{"Name1 (Alias1)", "Name2"},
			expected: []string{"Name1", "Alias1", "Name2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &actorSplitHandler{}
			fc := &model.FileContext{
				Meta: &model.MovieMeta{Actors: tt.actors},
			}
			err := h.Handle(context.Background(), fc)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, fc.Meta.Actors)
		})
	}
}

func TestTryExtractActor(t *testing.T) {
	h := &actorSplitHandler{}
	tests := []struct {
		name    string
		actor   string
		wantOK  bool
		wantLen int
	}{
		{"with alias", "Name (Alias)", true, 2},
		{"no parentheses", "Simple", false, 0},
		{"empty string", "", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := h.tryExtractActor(tt.actor)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Len(t, result, tt.wantLen)
			}
		})
	}
}

func TestCleanActor(t *testing.T) {
	h := &actorSplitHandler{}
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"fullwidth parens", "Test（Value）", "Test(Value)"},
		{"spaces trimmed", "  test  ", "test"},
		{"already clean", "clean", "clean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, h.cleanActor(tt.input))
		})
	}
}
