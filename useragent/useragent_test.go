package useragent

import "testing"

func TestListUA(t *testing.T) {
	for idx, ua := range defaultUserAgentList {
		t.Logf("list %d:=>%+v", idx, ua)
	}
	t.Logf("select one:%s", Select())
}
