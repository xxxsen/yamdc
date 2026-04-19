package medialib

import (
	"testing"

	projectgoleak "github.com/xxxsen/yamdc/internal/testsupport/goleak"
)

// TestMain 用 goleak 守护 medialib 包测试结束后不应残留任何 goroutine。
// TriggerFullSync / TriggerMove 会把实际工作 dispatch 到 bgWG 的后台 goroutine,
// 测试必须以 WaitBackground() 显式等待其退出; 任何测试路径没有 await 都会
// 在这里炸出来, 防止后台任务泄漏到下一个测试或进程退出时。
func TestMain(m *testing.M) {
	projectgoleak.VerifyTestMain(m)
}
