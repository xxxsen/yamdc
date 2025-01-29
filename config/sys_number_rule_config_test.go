package config

import (
	"testing"
	"yamdc/capture/ruleapi"

	"github.com/stretchr/testify/assert"
)

type testRewritePair struct {
	in  string
	out string
}

func TestNumberUncensorRule(t *testing.T) {
	tester := ruleapi.NewRegexpTester()
	err := tester.AddRules(sysNumberRule.NumberUncensorRules...)
	assert.NoError(t, err)
	trueList := []string{
		"112214_292",
		"112114-291",
		"n11451",
		"heyzo_1545",
		"hey-1111",
		"carib-11111-222",
		"22222-333",
		"010111-222",
		"H4610-Ki1111",
		"MKD-12345",
		"fc2-ppv-12345",
		"1pon-123",
		"smd-1234",
		"kb2134",
		"c0930-ki240528",
	}
	falseList := []string{
		"YMDS-164",
		"MBRBI-002",
		"LUKE-036",
		"SMDY-123",
	}
	for _, item := range trueList {
		ok, _ := tester.Test(item)
		assert.True(t, ok)
	}
	for _, item := range falseList {
		ok, _ := tester.Test(item)
		assert.False(t, ok)
	}
}

func TestFc2Rewrit(t *testing.T) {
	tests := []testRewritePair{
		{
			in:  "FC2-PPV-12345",
			out: "FC2-PPV-12345",
		},
		{
			in:  "fc2-ppv-12345",
			out: "FC2-PPV-12345",
		},
		{
			in:  "fc2ppv-12345-CD1",
			out: "FC2-PPV-12345-CD1",
		},
		{
			in:  "fc2ppv-12345-C-CD1",
			out: "FC2-PPV-12345-C-CD1",
		},
		{
			in:  "fc2ppv-123-asdasqwe2",
			out: "FC2-PPV-123-asdasqwe2",
		},
		{
			in:  "fc2",
			out: "fc2",
		},
		{
			in:  "aaa",
			out: "aaa",
		},
		{
			in:  "fc2-12345",
			out: "FC2-PPV-12345",
		},
		{
			in:  "fc2-123445-cd1",
			out: "FC2-PPV-123445-cd1",
		},
		{
			in:  "fc2ppv-123",
			out: "FC2-PPV-123",
		},
		{
			in:  "fc2_ppv_1234",
			out: "FC2-PPV-1234",
		},
		{
			in:  "fc2ppv_1234",
			out: "FC2-PPV-1234",
		},
	}

	rewriter := ruleapi.NewRegexpRewriter()
	for _, item := range sysNumberRule.NumberRewriteRules {
		err := rewriter.AddRules(ruleapi.RegexpRewriteRule{
			Rule:    item.Rule,
			Rewrite: item.Rewrite,
		})
		assert.NoError(t, err)
	}

	for _, tst := range tests {
		out, err := rewriter.Rewrite(tst.in)
		assert.NoError(t, err)
		assert.Equal(t, tst.out, out)
	}

}

func TestNumberAlphaNumberRewrite(t *testing.T) {
	tests := []testRewritePair{
		{
			in:  "123aaa-123434",
			out: "aaa-123434",
		},
		{
			in:  "aaa-1234-CD1",
			out: "aaa-1234-CD1",
		},
		{
			in:  "222aaa-22222_helloworld",
			out: "aaa-22222_helloworld",
		},
		{
			in:  "123abc_1234",
			out: "abc_1234",
		},
		{
			in:  "123abc_123aaa",
			out: "123abc_123aaa",
		},
	}
	rewriter := ruleapi.NewRegexpRewriter()
	for _, item := range sysNumberRule.NumberRewriteRules {
		err := rewriter.AddRules(ruleapi.RegexpRewriteRule{
			Rule:    item.Rule,
			Rewrite: item.Rewrite,
		})
		assert.NoError(t, err)
	}
	for _, tst := range tests {
		out, err := rewriter.Rewrite(tst.in)
		assert.NoError(t, err)
		assert.Equal(t, tst.out, out)
	}
}
