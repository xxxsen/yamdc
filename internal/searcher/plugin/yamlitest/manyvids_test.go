package yamlitest

import "testing"

func TestManyVidsParity(t *testing.T) {
	numbers := []string{"MANYVIDS-7274275", "MANYVIDS-1169588"}
	runPluginComparison(t, "manyvids", numbers)
}
