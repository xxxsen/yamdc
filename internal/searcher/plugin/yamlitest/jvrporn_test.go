package yamlitest

import "testing"

func TestJvrPornParity(t *testing.T) {
	numbers := []string{"jvr-100203", "jvr-100198"}
	runPluginComparison(t, "jvrporn", numbers)
}
