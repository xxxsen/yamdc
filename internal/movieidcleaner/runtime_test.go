package movieidcleaner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuntimeCleaner(t *testing.T) {
	tests := []struct {
		name      string
		inner     Cleaner
		expectPT  bool
	}{
		{
			name:     "nil_inner_uses_passthrough",
			inner:    nil,
			expectPT: true,
		},
		{
			name:     "non_nil_inner_is_used",
			inner:    &passthroughCleaner{},
			expectPT: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc := NewRuntimeCleaner(tc.inner)
			require.NotNil(t, rc)
			require.NotNil(t, rc.inner)
		})
	}
}

func TestRuntimeCleanerClean(t *testing.T) {
	rs := loadTestRuleSet(t)
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	rc := NewRuntimeCleaner(cl)

	tests := []struct {
		name       string
		input      string
		wantStatus Status
	}{
		{
			name:       "normal_match",
			input:      "ABC-123.mp4",
			wantStatus: StatusSuccess,
		},
		{
			name:       "no_match",
			input:      "pure-noise",
			wantStatus: StatusNoMatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := rc.Clean(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, res.Status)
		})
	}
}

func TestRuntimeCleanerExplain(t *testing.T) {
	rs := loadTestRuleSet(t)
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	rc := NewRuntimeCleaner(cl)

	tests := []struct {
		name       string
		input      string
		wantStatus Status
	}{
		{
			name:       "explain_match",
			input:      "ABC-123.mp4",
			wantStatus: StatusSuccess,
		},
		{
			name:       "explain_no_match",
			input:      "pure-noise",
			wantStatus: StatusNoMatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := rc.Explain(tc.input)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.NotNil(t, res.Final)
			assert.Equal(t, tc.wantStatus, res.Final.Status)
			assert.NotEmpty(t, res.Steps)
		})
	}
}

func TestRuntimeCleanerSwap(t *testing.T) {
	pt := NewPassthroughCleaner()
	rc := NewRuntimeCleaner(pt)

	res, err := rc.Clean("ABC-123.mp4")
	require.NoError(t, err)
	assert.Equal(t, StatusLowQuality, res.Status, "passthrough returns low quality")

	rs := loadTestRuleSet(t)
	cl, err := NewCleaner(rs)
	require.NoError(t, err)

	rc.Swap(cl)

	res, err = rc.Clean("ABC-123.mp4")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, res.Status, "swapped cleaner should match")
}

func TestRuntimeCleanerSwapNilIsNoop(t *testing.T) {
	pt := NewPassthroughCleaner()
	rc := NewRuntimeCleaner(pt)

	rc.Swap(nil)

	res, err := rc.Clean("ABC-123.mp4")
	require.NoError(t, err)
	assert.Equal(t, StatusLowQuality, res.Status, "nil swap should not change inner")
}
