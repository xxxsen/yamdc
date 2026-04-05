package yamlitest

import "testing"

func TestFC2Parity(t *testing.T) {
	numbers := []string{"FC2-PPV-4809303", "FC2-PPV-4869719"}
	runPluginComparison(t, "fc2", numbers)
}
