package handler

import (
	"context"
	"strings"

	"github.com/xxxsen/yamdc/internal/model"
)

// tagDedupHandler 负责对 MovieMeta.Genres 做 case-insensitive 去重,
// 冲突时优先保留大写字母更多的那个变体 ("4K" 胜 "4k"). 输出顺序按
// 各 lowercase key 的首次出现下标保留, 幂等.
//
// 设计意图: 作为 pipeline 末端的兜底清洁工, 不依赖 tag_mapper 等
// 可选 handler. 只做"去重 + 大小写归一", 不做 trim / normalize 等
// 超出"去重"职责的事.
type tagDedupHandler struct{}

// dedupPreferUpper 按 case-insensitive 去重, 冲突时保留大写字母数最多的变体.
//
// 规则:
//  1. key = strings.ToLower(tag), 同 key 视为"同一个 tag".
//  2. 同 key 冲突时, 取 ASCII 大写字母计数较多者
//     ("4K" score=1 胜 "4k" score=0,
//     "VR" score=2 胜 "Vr" score=1 胜 "vr" score=0).
//  3. 平票时, 保留首次出现的变体 (稳定, 不依赖 map 迭代顺序).
//  4. 输出顺序 = 各 lowercase key 首次出现的下标顺序, 保留原始语义.
//
// 纯 CJK tag ("字幕版") score 恒为 0, 不受影响; 纯重复也会被折成一条.
func dedupPreferUpper(tags []string) []string {
	if len(tags) == 0 {
		return tags
	}
	type pick struct {
		value string
		score int
	}
	best := make(map[string]*pick, len(tags))
	order := make([]string, 0, len(tags))
	for _, t := range tags {
		key := strings.ToLower(t)
		score := countUpperASCII(t)
		if p, ok := best[key]; ok {
			if score > p.score {
				p.value = t
				p.score = score
			}
			continue
		}
		best[key] = &pick{value: t, score: score}
		order = append(order, key)
	}
	rs := make([]string, 0, len(order))
	for _, k := range order {
		rs = append(rs, best[k].value)
	}
	return rs
}

// countUpperASCII 只数 ASCII 大写字母. 本项目里 tag 不会出现非 ASCII
// 拉丁字母 (Ä/ä 等), 不浪费 rune scan 成本.
func countUpperASCII(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			n++
		}
	}
	return n
}

func (h *tagDedupHandler) Handle(_ context.Context, fc *model.FileContext) error {
	if fc.Meta == nil || len(fc.Meta.Genres) == 0 {
		return nil
	}
	fc.Meta.Genres = dedupPreferUpper(fc.Meta.Genres)
	return nil
}

func init() {
	Register(HTagDedup, ToCreator(&tagDedupHandler{}))
}
