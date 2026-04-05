package yamlitest

import "testing"

func TestAvsoxParity(t *testing.T) {
	numbers := []string{"HEYZO-3806", "033126_01"}
	runPluginComparison(t, "avsox", numbers)
}
