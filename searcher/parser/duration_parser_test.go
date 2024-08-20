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
