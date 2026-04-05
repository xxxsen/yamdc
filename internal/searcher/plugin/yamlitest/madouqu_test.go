package yamlitest

import "testing"

func TestMadouquParity(t *testing.T) {
	numbers := []string{"MADOU-mgl-0011", "MADOU-xb2130"}
	runPluginComparison(t, "madouqu", numbers)
}
