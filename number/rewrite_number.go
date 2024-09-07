package number

import (
	"regexp"
	"strings"
	"unicode"
)

var defaultRewriteList = []iNumberRewriter{
	fc2NumberRewriter(),
	numberAlphaNumberRewriter(),
}

type onNumberRewriteCheckFunc func(str string) bool
type onNumberRewriteFunc func(str string) string

type iNumberRewriter interface {
	Check(str string) bool
	Rewrite(str string) string
}

type numberRewriter struct {
	onCheck   onNumberRewriteCheckFunc
	onRewrite onNumberRewriteFunc
}

func (c *numberRewriter) Check(str string) bool {
	if c.onCheck == nil {
		return true
	}
	return c.onCheck(str)
}

func (c *numberRewriter) Rewrite(str string) string {
	if c.onRewrite == nil {
		return str
	}
	return c.onRewrite(str)
}

func numberAlphaNumberRewriter() iNumberRewriter {
	checker := regexp.MustCompile(`^\d+[a-zA-Z]+-\d+`)
	//将324abc-234343之类的番号改写成abc-234343的形式(去掉前导数字)
	return &numberRewriter{
		onCheck: func(str string) bool {
			return checker.MatchString(str)
		},
		onRewrite: func(str string) string {
			for i, r := range str {
				if !unicode.IsDigit(r) {
					// 返回从第一个非数字字符开始的子字符串
					return str[i:]
				}
			}
			return str
		},
	}
}

func fc2NumberRewriter() iNumberRewriter {
	return &numberRewriter{
		onCheck: func(str string) bool {
			return strings.HasPrefix(str, "FC2")
		},
		onRewrite: func(str string) string {
			if !strings.Contains(str, "-PPV-") {
				str = strings.ReplaceAll(str, "FC2-", "FC2-PPV-")
			}
			str = strings.ReplaceAll(str, "FC2PPV-", "FC2-PPV-")
			return str
		},
	}
}

func rewriteNumber(str string) string {
	for _, rewriter := range defaultRewriteList {
		if !rewriter.Check(str) {
			continue
		}
		str = rewriter.Rewrite(str)
	}
	return str
}
