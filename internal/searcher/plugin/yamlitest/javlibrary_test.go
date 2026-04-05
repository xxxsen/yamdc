package yamlitest

import "testing"

func TestJavLibraryParity(t *testing.T) {
	numbers := []string{}
	runPluginComparison(t, "javlibrary", numbers)
}
