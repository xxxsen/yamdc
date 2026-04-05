package yamlitest

import "testing"

func TestJav321Parity(t *testing.T) {
	numbers := []string{"jufe-618", "snos-174", "gana-3366"}
	runPluginComparison(t, "jav321", numbers)
}
