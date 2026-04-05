package yamlitest

import "testing"

func Test18AVParity(t *testing.T) {
	numbers := []string{"REBD-1022", "ATID-671"}
	runPluginComparison(t, "18av", numbers)
}
