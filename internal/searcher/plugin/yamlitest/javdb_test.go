package yamlitest

import "testing"

func TestJavDBParity(t *testing.T) {
	numbers := []string{"STOUCH-173", "STBD-203"}
	runPluginComparison(t, "javdb", numbers)
}
