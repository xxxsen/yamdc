package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectDomain_Empty(t *testing.T) {
	s, ok := SelectDomain(nil)
	assert.False(t, ok)
	assert.Empty(t, s)

	s, ok = SelectDomain([]string{})
	assert.False(t, ok)
	assert.Empty(t, s)
}

func TestSelectDomain_Single(t *testing.T) {
	s, ok := SelectDomain([]string{"only.example"})
	require.True(t, ok)
	assert.Equal(t, "only.example", s)
}

func TestSelectDomain_Multiple(t *testing.T) {
	in := []string{"a.example", "b.example", "c.example"}
	s, ok := SelectDomain(in)
	require.True(t, ok)
	assert.Contains(t, in, s)
}

func TestMustSelectDomain(t *testing.T) {
	assert.Equal(t, "x", MustSelectDomain([]string{"x"}))
	assert.Panics(t, func() {
		MustSelectDomain(nil)
	})
}
