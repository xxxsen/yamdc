package yamlitest

import "testing"

func TestCospuriParity(t *testing.T) {
	numbers := []string{"COSPURI-0419YA2D"}
	runPluginComparison(t, "cospuri", numbers)
}
