package number

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testRewritePair struct {
	in    string
	match bool
	out   string
}

func TestRewriteFc2(t *testing.T) {
	tests := []testRewritePair{
		{
			in:    "FC2-PPV-12345",
			match: true,
			out:   "FC2-PPV-12345",
		},
		{
			in:    "aaa",
			match: false,
			out:   "aaa",
		},
		{
			in:    "fc2-12345",
			match: true,
			out:   "fc2-ppv-12345",
		},
		{
			in:    "fc2ppv-123",
			match: true,
			out:   "fc2-ppv-123",
		},
	}
	fc2Rewriter := fc2NumberRewriter()
	for _, tst := range tests {
		tst.in = strings.ToUpper(tst.in)
		tst.out = strings.ToUpper(tst.out)
		assert.Equal(t, tst.match, fc2Rewriter.Check(tst.in))
		if !tst.match {
			continue
		}
		out := fc2Rewriter.Rewrite(tst.in)
		assert.Equal(t, tst.out, out)
	}
}

func TestWriteNumberAlphaNumberFormat(t *testing.T) {
	tests := []testRewritePair{
		{
			in:    "123aaa-123434",
			match: true,
			out:   "aaa-123434",
		},
		{
			in:    "aaa-1234",
			match: false,
			out:   "aaa-1234",
		},
		{
			in:    "222aaa-22222_helloworld",
			match: true,
			out:   "aaa-22222_helloworld",
		},
	}
	rewriter := numberAlphaNumberRewriter()
	for _, tst := range tests {
		tst.in = strings.ToUpper(tst.in)
		tst.out = strings.ToUpper(tst.out)
		assert.Equal(t, tst.match, rewriter.Check(tst.in))
		if !tst.match {
			continue
		}
		out := rewriter.Rewrite(tst.in)
		assert.Equal(t, tst.out, out)
	}
}
