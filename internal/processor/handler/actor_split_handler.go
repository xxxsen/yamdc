package handler

import (
	"context"
	"regexp"
	"strings"
	"github.com/xxxsen/yamdc/internal/model"
)

var (
	defaultExtractActorRegexp = regexp.MustCompile(`\s*(.+?)\s*\(\s*(.+?)\s*\)`)
)

type actorSplitHandler struct {
}

func (h *actorSplitHandler) cleanActor(actor string) string {
	actor = strings.TrimSpace(actor)
	actor = strings.ReplaceAll(actor, "（", "(")
	actor = strings.ReplaceAll(actor, "）", ")")
	return actor
}

func (h *actorSplitHandler) tryExtractActor(actor string) ([]string, bool) {
	// 查找所有匹配的内容
	matches := defaultExtractActorRegexp.FindAllStringSubmatch(actor, -1)

	if len(matches) == 0 {
		return nil, false
	}
	rs := make([]string, 0, 2)
	for _, match := range matches {
		if len(match) == 3 { // match[0] 是整个匹配的字符串，match[1] 和 match[2] 是捕获组
			rs = append(rs, strings.TrimSpace(match[1]), strings.TrimSpace(match[2]))
		}
	}
	return rs, true
}

func (h *actorSplitHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	//如果女优有括号, 尝试将其从括号中提取出来, example: 永野司 (永野つかさ)
	actorlist := make([]string, 0, len(fc.Meta.Actors))
	for _, actor := range fc.Meta.Actors {
		actor = h.cleanActor(actor)
		splited, ok := h.tryExtractActor(actor)
		if !ok {
			actorlist = append(actorlist, actor)
			continue
		}
		actorlist = append(actorlist, splited...)
	}
	fc.Meta.Actors = actorlist
	return nil
}

func init() {
	Register(HActorSpliter, HandlerToCreator(&actorSplitHandler{}))
}
