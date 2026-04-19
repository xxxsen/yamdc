package job

import (
	"testing"

	projectgoleak "github.com/xxxsen/yamdc/internal/testsupport/goleak"
)

// TestMain 用 goleak 守护 job 包测试结束后不应残留任何 goroutine。
// NewService 会常驻一个 runWorker goroutine, 必须用 Stop() 显式收敛 (关闭
// queue 才能让 <-s.queue 退出); 任何测试漏调 Stop 都会让这只 goroutine 悬挂,
// goleak 在测试进程退出前将其打出, 防止隐藏的 goroutine 泄漏流进生产。
func TestMain(m *testing.M) {
	projectgoleak.VerifyTestMain(m)
}
