// Package goleak 封装 go.uber.org/goleak 在本项目的落地方式: 每个 goroutine
// 密集包 (job / medialib / cronscheduler / scanner ...) 的 TestMain 统一调
// VerifyTestMain, 在单测退出前检查是否残留 goroutine, 把 "忘了调 Stop /
// Wait" 的测试在 CI 层直接拦住。
//
// 之所以再包一层而不是直接 import goleak, 是因为本项目有两类 "框架级" 长驻
// goroutine 天然无法 Close, 会被 goleak 报成误伤, 需要全局统一豁免:
//
//  1. gopkg.in/natefinch/lumberjack.v2 的 millRun: logutil.Init 会创建一个
//     进程级 lumberjack.Logger, 每次文件滚动会起一个 mill goroutine 阻塞在
//     chan receive 上, 直到 Logger.Close() 才退出; 但 logutil 没有暴露关闭
//     接口, 测试进程结束时它必然悬挂。不是我们的 bug, 不应干扰告警。
//
// 其它包自带的 "必然泄漏" 栈顶函数按同样方式往 defaultIgnores 里加即可, 请
// 同步在注释里解释为什么豁免, 避免变成无差别屏蔽 goleak 的口子。
package goleak

import (
	"testing"

	"go.uber.org/goleak"
)

// defaultIgnores 列出项目所有包共享的 "已知框架级长驻" goroutine 栈顶函数。
// 新条目必须满足: (1) 源头是第三方库; (2) 无法从应用层关闭; (3) 已验证不会
// 导致生产侧资源泄漏 (常见于日志器 / prometheus collector 等 process-level
// 单例)。
// NOTE: Go 的 runtime 把 import path 里的 "." 用 "%2e" 表示 (乃至于在 panic
// stack 里也是这种形式), 因此 lumberjack.v2 的栈顶函数名是
// "gopkg.in/natefinch/lumberjack%2ev2.(*Logger).millRun" 而不是
// "gopkg.in/natefinch/lumberjack.v2.(*Logger).millRun"。goleak.IgnoreTopFunction
// 做的是精确字符串匹配, 写错编码就会静默失效。
var defaultIgnores = []goleak.Option{
	goleak.IgnoreTopFunction("gopkg.in/natefinch/lumberjack%2ev2.(*Logger).millRun"),
}

// VerifyTestMain 提供与 goleak.VerifyTestMain 等价的入口, 但附带项目级默认
// 豁免。调用点形如:
//
//	func TestMain(m *testing.M) {
//		projectgoleak.VerifyTestMain(m)
//	}
func VerifyTestMain(m *testing.M, extra ...goleak.Option) {
	opts := make([]goleak.Option, 0, len(defaultIgnores)+len(extra))
	opts = append(opts, defaultIgnores...)
	opts = append(opts, extra...)
	goleak.VerifyTestMain(m, opts...)
}
