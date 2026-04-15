package capture

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildAuthorsName(t *testing.T) {
	tests := []struct {
		name     string
		acts     []string
		expected string
	}{
		{
			name:     "empty list",
			acts:     []string{},
			expected: defaultNonActorName,
		},
		{
			name:     "nil list",
			acts:     nil,
			expected: defaultNonActorName,
		},
		{
			name:     "single actor",
			acts:     []string{"Alice"},
			expected: "Alice",
		},
		{
			name:     "two actors",
			acts:     []string{"hello", "world"},
			expected: "hello,world",
		},
		{
			name:     "at limit (3 actors)",
			acts:     []string{"1", "2", "3"},
			expected: defaultMultiActorAsName,
		},
		{
			name:     "above limit (5 actors)",
			acts:     []string{"1", "2", "3", "4", "5"},
			expected: defaultMultiActorAsName,
		},
		{
			name:     "character limit truncation",
			acts:     []string{strings.Repeat("a", 150), strings.Repeat("b", 100)},
			expected: strings.Repeat("a", 150) + ",",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAuthorsName(tt.acts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "short title",
			title:    "short",
			expected: "short",
		},
		{
			name:     "empty title",
			title:    "",
			expected: "",
		},
		{
			name:     "at limit",
			title:    strings.Repeat("x", defaultMaxItemCharactor),
			expected: strings.Repeat("x", defaultMaxItemCharactor),
		},
		{
			name:     "above limit truncated",
			title:    strings.Repeat("x", defaultMaxItemCharactor+50),
			expected: strings.Repeat("x", defaultMaxItemCharactor),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTitle(tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}
