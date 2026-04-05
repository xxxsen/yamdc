package yamlitest

import "testing"

func TestJavhooParity(t *testing.T) {
	numbers := []string{"ABF-326", "JERA-028"}
	runPluginComparison(t, "javhoo", numbers)
}
