package handler

import (
	"context"
	"strings"
	"unicode"
	"github.com/xxxsen/yamdc/internal/model"

	"github.com/samber/lo"
)

type tagPadderHandler struct{}

func (h *tagPadderHandler) generateNumberPrefixTag(fc *model.FileContext) (string, bool) {
	//将番号的前版本部分提取, 作为分类的一部分, 方便从一个影片看到这个系列相关的全部影片
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

func (h *tagPadderHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	//提取番号特有的tag
	fc.Meta.Genres = append(fc.Meta.Genres, fc.Number.GenerateTags()...)
	//提取番号前缀作为tag
	if tag, ok := h.generateNumberPrefixTag(fc); ok {
		h.rewriteOrAppendTag(fc.Meta, tag)
	}
	fc.Meta.Genres = lo.Uniq(fc.Meta.Genres)
	return nil
}

func init() {
	Register(HTagPadder, HandlerToCreator(&tagPadderHandler{}))
}
