package yamlitest

import "testing"

func TestCaribprParity(t *testing.T) {
	numbers := []string{"010825_001", "042613_001", "040226_001"}
	runPluginComparison(t, "caribpr", numbers)
}
