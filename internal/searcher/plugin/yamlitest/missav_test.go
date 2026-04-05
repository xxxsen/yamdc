package yamlitest

import "testing"

func TestMissavParity(t *testing.T) {
	numbers := []string{"URKN-012", "YMSR-097"}
	runPluginComparison(t, "missav", numbers)
}
