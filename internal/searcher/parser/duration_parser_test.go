package parser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testPair struct {
	in  string
	sec int64
}

func TestHHMMSS(t *testing.T) {
	tests := []testPair{
		{in: "01    :01:    01", sec: 1*3600 + 60 + 1},
		{in: "02:   05", sec: 2*60 + 5},
	}
	for _, tst := range tests {
		out := DefaultHHMMSSDurationParser(context.Background())(tst.in)
		assert.Equal(t, tst.sec, out)
	}
}

func TestConv(t *testing.T) {
	sts := []struct {
		in  string
		out int64
	}{
		{"47分钟", 47 * 60},
		{" 10分钟", 600},
		{"140分", 140 * 60},
		{"117分鐘", 117 * 60},
	}
	for _, st := range sts {
		out, err := toDuration(st.in)
		assert.NoError(t, err)
		assert.Equal(t, st.out, out)
	}
}

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
