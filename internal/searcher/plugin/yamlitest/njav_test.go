package yamlitest

import "testing"

func TestNJavParity(t *testing.T) {
	numbers := []string{}
	runPluginComparison(t, "njav", numbers)
}
