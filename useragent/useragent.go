package useragent

import (
	"fmt"
	"math/rand"
	"strings"
)

var (
	defaultUserAgentList = make([]string, 0, 20)
)

const (
	defaultMaxBrowserVersion = 126
)

var uaTemplateList = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/{BROWSER_VERSION}.0.0.0 Safari/537.3",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/{BROWSER_VERSION}.0.0.0 Safari/537.36 Edg/{BROWSER_VERSION}.0.0.",
}

const (
	defaultUa = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.3"
)

func Select() string {
	if len(defaultUserAgentList) == 0 {
		return defaultUa
	}
	return defaultUserAgentList[int(rand.Int31())%len(defaultUserAgentList)]
}

func init() {
	for i := defaultMaxBrowserVersion - 5; i <= defaultMaxBrowserVersion; i++ {
		for _, tpl := range uaTemplateList {
			defaultUserAgentList = append(defaultUserAgentList, strings.ReplaceAll(tpl, "{BROWSER_VERSION}", fmt.Sprintf("%d", i)))
		}
	}
}
