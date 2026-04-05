package yamlitest

import "testing"

func TestFreeJavBtParity(t *testing.T) {
	numbers := []string{}
	runPluginComparison(t, "freejavbt", numbers)
}
