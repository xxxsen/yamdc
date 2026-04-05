package yamlitest

import "testing"

func TestAiravParity(t *testing.T) {
	numbers := []string{"300MIUM-1367", "SIRO-5637"}
	runPluginComparison(t, "airav", numbers)
}
