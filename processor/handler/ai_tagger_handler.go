package handler

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
	"yamdc/aiengine"
	"yamdc/model"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

const (
	defaultAITaggerPrompt = `
你需要从下面的这些信息中提取5个最合适的中文标签, 使用逗号将其隔开, 产生的标签不能重复也不能与给到的现有标签重复, 如果无法产生有效标签, 则返回空文本, 不需要有多余的解释和输出。

标题:"{TITLE}"
摘要:"{PLOT}"
`
)

const (
	defaultMaxAllowAITagLength        = 4
	defaultMinTitleLengthForAITagging = 20
	defualtMinPlotLengthForAITagging  = 60
)

type aiTaggerHandler struct {
}

func (a *aiTaggerHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	if !aiengine.IsAIEngineEnabled() {
		return nil
	}
	title := fc.Meta.Title
	if len(fc.Meta.TitleTranslated) > 0 {
		title = fc.Meta.TitleTranslated
	}
	plot := fc.Meta.Plot
	if len(fc.Meta.PlotTranslated) > 0 {
		plot = fc.Meta.PlotTranslated
	}
	if len(title) < defaultMinTitleLengthForAITagging && len(plot) < defualtMinPlotLengthForAITagging {
		return nil
	}
	res, err := aiengine.Complete(ctx, defaultAITaggerPrompt, map[string]interface{}{
		"TITLE": title,
		"PLOT":  plot,
	})
	if err != nil {
		return fmt.Errorf("call ai engine for tagging failed, err:%w", err)
	}
	taglist := strings.Split(res, ",")
	for _, tag := range taglist {
		if utf8.RuneCountInString(tag) > defaultMaxAllowAITagLength {
			logutil.GetLogger(ctx).Warn("warning: tag is too long, may has err in ai engine, ignore it", zap.String("tag", tag))
			continue
		}
		fc.Meta.Genres = append(fc.Meta.Genres, "AI-"+strings.TrimSpace(tag))
	}
	return nil
}

func init() {
	Register(HAITagger, HandlerToCreator(&aiTaggerHandler{}))
}
