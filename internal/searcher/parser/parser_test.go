package parser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDateOnlyReleaseDateParser(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int64
	}{
		{name: "valid_date", input: "2024-01-02", expect: 1704153600000},
		{name: "invalid_date", input: "not-a-date", expect: 0},
		{name: "empty", input: "", expect: 0},
		{name: "partial_date", input: "2024-01", expect: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := DateOnlyReleaseDateParser(context.Background())
			result := fn(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestDefaultMMDurationParser(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int64
	}{
		{name: "valid_minutes", input: "120", expect: 7200},
		{name: "zero", input: "0", expect: 0},
		{name: "non_numeric", input: "abc", expect: 0},
		{name: "empty", input: "", expect: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := DefaultMMDurationParser(context.Background())
			result := fn(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestDefaultHHMMSSDurationParser(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int64
	}{
		{name: "hh_mm_ss", input: "01:01:01", expect: 3661},
		{name: "mm_ss", input: "02:05", expect: 125},
		{name: "ss_only", input: "30", expect: 30},
		{name: "too_many_parts", input: "1:2:3:4", expect: 0},
		{name: "invalid_number", input: "01:ab", expect: 0},
		{name: "with_spaces", input: "01    :01:    01", expect: 3661},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := DefaultHHMMSSDurationParser(context.Background())
			result := fn(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestDefaultDurationParser(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int64
	}{
		{name: "with_unit", input: "47分钟", expect: 47 * 60},
		{name: "with_unit_2", input: "120分", expect: 120 * 60},
		{name: "invalid", input: "abc", expect: 0},
		{name: "empty", input: "", expect: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := DefaultDurationParser(context.Background())
			result := fn(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestMinuteOnlyDurationParser(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int64
	}{
		{name: "valid", input: "90", expect: 5400},
		{name: "zero", input: "0", expect: 0},
		{name: "non_numeric", input: "xyz", expect: 0},
		{name: "empty", input: "", expect: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := MinuteOnlyDurationParser(context.Background())
			result := fn(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestToDuration_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expect  int64
		wantErr bool
	}{
		{name: "valid", input: "47分钟", expect: 47 * 60},
		{name: "no_match", input: "abc", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expect, result)
			}
		})
	}
}

func TestHumanDurationToSecond_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int64
	}{
		{name: "all_units", input: "1h30m45s", expect: 3600 + 1800 + 45},
		{name: "empty", input: "", expect: 0},
		{name: "no_unit", input: "123", expect: 0},
		{name: "only_hours", input: "2h", expect: 7200},
		{name: "only_seconds", input: "59s", expect: 59},
		{name: "non_numeric_chars", input: "abc1h", expect: 3600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HumanDurationToSecond(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}
