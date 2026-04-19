package cronscheduler

import (
	"testing"

	projectgoleak "github.com/xxxsen/yamdc/internal/testsupport/goleak"
)

// TestMain 用 goleak 守护 cronscheduler 包测试结束后不应残留任何 goroutine。
// scheduler 自身的 Start/Stop 必须严格配对; 一旦 Stop 漏调或 robfig/cron 的
// 调度 goroutine 没能在 stopTimeout 内退出, goleak 会在 test 二进制退出前直接
// 把泄漏的栈打出来, 在 CI 层把问题拦在单测阶段。
func TestMain(m *testing.M) {
	projectgoleak.VerifyTestMain(m)
}
