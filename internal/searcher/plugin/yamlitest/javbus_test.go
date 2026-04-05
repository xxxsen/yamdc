package yamlitest

import "testing"

func TestJavBusParity(t *testing.T) {
	numbers := []string{"ABF-333", "DRPT-108", "PASN-016", "MIAB-646"}
	runPluginComparison(t, "javbus", numbers)
}
