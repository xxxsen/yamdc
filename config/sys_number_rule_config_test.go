package config

import (
	"strings"
	"testing"
	"yamdc/capture/ruleapi"

	"github.com/stretchr/testify/assert"
)

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
		item = strings.ToUpper(item)
		ok, _ := tester.Test(item)
		assert.True(t, ok)
	}
	for _, item := range falseList {
		item = strings.ToUpper(item)
		ok, _ := tester.Test(item)
		assert.False(t, ok)
	}
}
