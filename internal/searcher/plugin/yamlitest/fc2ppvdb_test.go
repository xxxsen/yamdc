package yamlitest

import "testing"

func TestFC2PPVDBParity(t *testing.T) {
	numbers := []string{"FC2-PPV-4863849", "FC2-PPV-4869719"}
	runPluginComparison(t, "fc2ppvdb", numbers)
}
