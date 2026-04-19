package handler

import (
	"context"
	"strings"
	"unicode"

	"github.com/samber/lo"

	"github.com/xxxsen/yamdc/internal/model"
)

type tagPadderHandler struct{}

func (h *tagPadderHandler) generateNumberPrefixTag(fc *model.FileContext) (string, bool) {
	// 将影片 ID 的前缀部分提取为分类标签, 方便查看同系列影片
	sb := strings.Builder{}
	isPureNumber := true
	for _, c := range fc.Number.GetNumberID() {
		if c == '-' || c == '_' {
			break
		}
		if unicode.IsLetter(c) {
			isPureNumber = false
		}
		sb.WriteRune(c)
	}
	if isPureNumber {
		return "", false
	}
	return sb.String(), true
}

func (h *tagPadderHandler) rewriteOrAppendTag(fc *model.MovieMeta, tagname string) {
	isContained := false
	for idx, item := range fc.Genres {
		if strings.EqualFold(item, tagname) {
			fc.Genres[idx] = tagname
			isContained = true
		}
	}
	if isContained {
		return
	}
	fc.Genres = append(fc.Genres, tagname)
}

func (h *tagPadderHandler) Handle(_ context.Context, fc *model.FileContext) error {
	// 提取影片 ID 派生标签
	fc.Meta.Genres = append(fc.Meta.Genres, fc.Number.GenerateTags()...)
	// 提取影片 ID 前缀作为标签
	if tag, ok := h.generateNumberPrefixTag(fc); ok {
		h.rewriteOrAppendTag(fc.Meta, tag)
	}
	fc.Meta.Genres = lo.Uniq(fc.Meta.Genres)
	return nil
}

func init() {
	Register(HTagPadder, ToCreator(&tagPadderHandler{}))
}
