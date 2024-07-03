package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		out, err := ToDuration(st.in)
		assert.NoError(t, err)
		assert.Equal(t, st.out, out)
	}
}
