package flarerr

import (
	"net/http"
	"testing"
	"time"
	"yamdc/client"

	"github.com/stretchr/testify/assert"
)

func TestByPas(t *testing.T) {
	c, _ := New(&http.Client{}, "http://127.0.0.1:8191")
	MustAddToSolverList(c, "www.javlibrary.com")
	req, err := http.NewRequest(http.MethodGet, "https://www.javlibrary.com/cn/vl_searchbyid.php?keyword=ZMAR-134", nil)
	assert.NoError(t, err)
	start := time.Now()
	rsp, err := c.Do(req)
	assert.NoError(t, err)
	raw, err := client.ReadHTTPData(rsp)
	assert.NoError(t, err)
	t.Logf("cost:%dms", time.Since(start).Milliseconds())
	t.Logf("read data:%s", string(raw))
}
