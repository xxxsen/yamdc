package yamlitest

import "testing"

func TestTKTubeParity(t *testing.T) {
	numbers := []string{"ADN-752", "ADN-721", "ATID-671"}
	runPluginComparison(t, "tktube", numbers)
}
