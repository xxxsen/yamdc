package client

import (
	"fmt"
	"net/http/cookiejar"
)

// MustCookieJar 构造一个内存 cookie jar (不带 public suffix list)。
// 标准库的 cookiejar.New(nil) 当前不会返回 error, 但显式 panic 胜过
// 静默丢弃 —— 如果未来 stdlib 语义变化, 构造失败会立即暴露而不是
// 留下一个 nil jar 引发后续莫名其妙的空指针或行为。
func MustCookieJar() *cookiejar.Jar {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Sprintf("cookiejar.New(nil) returned error: %v", err))
	}
	return jar
}
