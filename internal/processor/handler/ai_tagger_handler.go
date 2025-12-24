package handler

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/model"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

const (
	defaultAITaggerPrompt = `
You are an expert in tagging adult video content. The input is a title or description written in Chinese, Japanese, or English. Your task is to extract up to 5 keywords that are explicitly mentioned or directly implied by the text. Do not guess or invent.

Each keyword must:
- Be in Simplified Chinese
- Be 2 to 3 Chinese characters long (no 4-character or longer phrases)
- Be directly supported by the text

Only output the keywords, separated by commas. No explanation, no extra text.

Input:
TITLE: "{TITLE}"
PLOT: "{PLOT}"
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
	if utf8.RuneCountInString(title) < defaultMinTitleLengthForAITagging && utf8.RuneCountInString(plot) < defualtMinPlotLengthForAITagging {
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
