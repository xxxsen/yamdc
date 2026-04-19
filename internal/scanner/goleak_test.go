package scanner

import (
	"testing"

	projectgoleak "github.com/xxxsen/yamdc/internal/testsupport/goleak"
)

// TestMain 用 goleak 守护 scanner 包测试结束后不应残留任何 goroutine。
// Scanner.Scan 是同步遍历, 自身不起 goroutine; 但调用方侧 (web handler / cron
// job) 将来再封 goroutine 时, 这道守门能把 "忘了等 cleanup" 的实现漏洞就地拦住。
func TestMain(m *testing.M) {
	projectgoleak.VerifyTestMain(m)
}
