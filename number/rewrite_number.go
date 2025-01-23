package number

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"yamdc/config"

	"github.com/dlclark/regexp2"
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

/* 读取配置RegexesToReplace,用来替换或者移除无关字段 */
func replaceWithRegexes(str string) string {
	cfg := config.GetConfig()
	newStr := str
	for _, regex_to_replace := range cfg.RegexesToReplace {
		// 使用三方regex2 才能 前瞻/后顾断言
		re, err := regexp2.Compile(regex_to_replace[0], regexp2.None)
		if err != nil {
			fmt.Println("正则错误:", err)
		}
		repl := regex_to_replace[1]
		// 打印替换后的字符串

		newStr, err = re.Replace(newStr, repl, -1, -1)
		if err != nil {
			fmt.Println("替换错误:", err)
		}

	}
	return newStr
}
