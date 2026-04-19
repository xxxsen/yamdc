package movieidcleaner

import (
	"strings"
	"testing"
)

// FuzzCleanWithDefaultBundle 把默认 bundle 规则集灌入 cleaner, 然后用任意字节
// 串打。该路径覆盖 normalizer / rewrite / suffix / noise / matcher / post-processor
// 的全链路。
//
// 不变量:
//  1. Clean / Explain 对任意输入都不 panic; 当前实现 Clean 始终返回 nil error,
//     Explain 也只在最内层触发 number.Parse 失败时返回 status=low_quality.
//  2. 即便拿到 StatusNoMatch, RawInput 必须与输入字节完全相等 (我们不允许规则
//     集侧偷偷把 raw input 改写后再返回, 这会破坏上游做 audit log 的前提)。
//  3. Normalized / NumberID 在 StatusSuccess 时必须为非空且去除了所有空白 —
//     这是下游 job / medialib 侧索引 / 去重 / 文件重命名的硬前置条件。
//  4. Suffixes 数组每个元素非空; 若出现 "" 元素会让 post processor 的 append
//     "-" 逻辑拼出 "ID--X" 这种非法字符串。
//
// 调用方式 (CI 默认只跑 seed, 不启动 fuzzing 循环):
//
//	go test -run=^$ -fuzz='^FuzzCleanWithDefaultBundle$' -fuzztime=30s \
//	    ./internal/movieidcleaner/
func FuzzCleanWithDefaultBundle(f *testing.F) {
	rs := mustLoadDefaultBundle(f)
	cl, err := NewCleaner(rs)
	if err != nil {
		f.Fatalf("NewCleaner failed with default bundle: %v", err)
	}

	seeds := []string{
		"",
		"[VID] rawxppv12345 sub.mp4",
		"abc123 disc2.avi",
		"www.example.com OPEN-1234 leak.mp4",
		"ABC_123.mp4",
		"pure-noise-file-name",
		"foo.bar",
		"/abs/path/file.mp4",
		".hidden",
		strings.Repeat("A", 512),
		"中文-123",
		"\x00\x01\x02",
		"\x81",
		"\ufffd",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		res, err := cl.Clean(input)
		if err != nil {
			t.Fatalf("Clean returned error %v for %q (signature allows it but impl should never)", err, input)
		}
		if res == nil {
			t.Fatalf("Clean returned nil Result for %q", input)
		}
		if res.RawInput != input {
			t.Fatalf("RawInput mutated: got %q, want %q", res.RawInput, input)
		}
		for i, suf := range res.Suffixes {
			if suf == "" {
				t.Fatalf("Suffixes[%d] is empty for input %q; full=%v", i, input, res.Suffixes)
			}
		}
		if res.Status == StatusSuccess {
			if res.NumberID == "" {
				t.Fatalf("StatusSuccess but NumberID is empty for %q", input)
			}
			if res.Normalized == "" {
				t.Fatalf("StatusSuccess but Normalized is empty for %q", input)
			}
			if strings.ContainsAny(res.Normalized, " \t\n\r") {
				t.Fatalf("Normalized must not contain whitespace, got %q for %q", res.Normalized, input)
			}
		}
		// Explain 走另一条路径 (显式收集 steps), 同样不能 panic。
		ex, err := cl.Explain(input)
		if err != nil {
			t.Fatalf("Explain returned error %v for %q", err, input)
		}
		if ex == nil || ex.Final == nil {
			t.Fatalf("Explain returned nil (or nil Final) for %q", input)
		}
	})
}

func mustLoadDefaultBundle(tb testing.TB) *RuleSet {
	tb.Helper()
	rs, err := LoadRuleSetFromPath("testdata/default-bundle")
	if err != nil {
		tb.Fatalf("load default bundle: %v", err)
	}
	return rs
}
