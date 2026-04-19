package number

import (
	"strings"
	"testing"
)

// FuzzParse 覆盖 Parse 的健壮性边界 — 所有文件名解析路径的入口函数。
// 不变量:
//  1. 绝不 panic (任何输入都要走完 resolveSuffixInfo 循环)。
//  2. 若返回 err != nil 则 Number 指针必为 nil (避免 "error + partial result"
//     这种容易被上游漏判的病态返回)。
//  3. 若返回 Number 非 nil, 则其 NumberID 必须是原始 upper(str) 的前缀 —
//     所有 suffix 解析只从末尾裁剪, 不会改写中间字符。
//  4. MultiCDIndex >= 0 — 负数或溢出说明 strconv/切片逻辑有锅。
//
// 用 go test -run=^$ -fuzz=FuzzParse -fuzztime=30s ./internal/number/ 启动。
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"DEMO-3332",
		"052624_01",
		"052624_01-CD2",
		"abc-leak-c",
		"xyz-8k-vr",
		"hack1-u",
		"UHD-2160P",
		"k0009-c_cd1-4k",
		"-c-4k",
		"A",
		"_-_",
		"CD99999999999999999999",
		strings.Repeat("-CD1", 50),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, str string) {
		// 不变量 0: 不能因为任何输入让 Parse panic。
		n, err := Parse(str)
		if err != nil {
			if n != nil {
				t.Fatalf("Parse returned both err=%v and non-nil Number for %q", err, str)
			}
			return
		}
		if n == nil {
			t.Fatalf("Parse returned nil Number and nil error for %q", str)
		}

		upper := strings.ToUpper(str)
		id := n.GetNumberID()
		if !strings.HasPrefix(upper, id) {
			t.Fatalf("numberID %q is not a prefix of upper(input)=%q", id, upper)
		}
		if n.GetMultiCDIndex() < 0 {
			t.Fatalf("multiCDIndex must be non-negative, got %d for %q", n.GetMultiCDIndex(), str)
		}
	})
}

// FuzzParseWithFileName 覆盖 ParseWithFileName 的扩展名 / 路径分隔符路径。
// 主要防回归: filepath.Base / Ext 对无后缀 / ".hidden" / 多重扩展名的处理不
// 应导致下游 Parse 接收越界切片或 panic。
func FuzzParseWithFileName(f *testing.F) {
	seeds := []string{
		"DEMO-3332.mp4",
		"a.mp4",
		".mp4",
		"noext",
		"/abs/path/abc-123.mp4",
		"dir/sub/xyz.txt",
		"a/b/c/",
		"file.tar.gz",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(_ *testing.T, p string) {
		// 只检查不 panic; 很多合法文件名 (".mp4", "noext") 本来就会回 err,
		// 这里只保证分支走完不 crash。
		_, _ = ParseWithFileName(p)
	})
}

// FuzzGetCleanID 验证 "-" / "_" 剔除逻辑永远不会多留或少留字符。
// 不变量:
//  1. 结果长度 = 原长度 - ('-' 计数 + '_' 计数)。
//  2. 结果不含 '-' 或 '_'。
//  3. 其它字符顺序保持 (GetCleanID 不该重排 rune)。
func FuzzGetCleanID(f *testing.F) {
	seeds := []string{"", "abc-123", "a-b_c_d", "-_-", "纯中文", "abc\u4e00123"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := GetCleanID(s)
		want := strings.Count(s, "-") + strings.Count(s, "_")
		if len(s)-len(got) != want {
			t.Fatalf("GetCleanID(%q)=%q: removed %d bytes, want %d",
				s, got, len(s)-len(got), want)
		}
		if strings.ContainsAny(got, "-_") {
			t.Fatalf("GetCleanID(%q)=%q still contains - or _", s, got)
		}
		// 把原字符串里的 - 和 _ 都抹掉后应等于 got。
		sanitized := strings.NewReplacer("-", "", "_", "").Replace(s)
		if sanitized != got {
			t.Fatalf("GetCleanID(%q)=%q, expected %q (order-preserving strip)",
				s, got, sanitized)
		}
	})
}
