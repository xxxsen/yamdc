package utils

import (
	"testing"
)

func TestHumanDurationToSecond(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{
			input:    "01h16m56s",
			expected: 1*3600 + 16*60 + 56, // 5816 seconds
		},
		{
			input:    "1m56s",
			expected: 1*60 + 56, // 116 seconds
		},
		{
			input:    "12s",
			expected: 12, // 12 seconds
		},
		{
			input:    "2h30m",
			expected: 2*3600 + 30*60, // 9000 seconds
		},
		{
			input:    "45m",
			expected: 45 * 60, // 2700 seconds
		},
	}

	for _, tt := range tests {
		result := HumanDurationToSecond(tt.input)
		if result != tt.expected {
			t.Errorf("HumanDurationToSecond(%s) = %d; expected %d", tt.input, result, tt.expected)
		}
	}
}
