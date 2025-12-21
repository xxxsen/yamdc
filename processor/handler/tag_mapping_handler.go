package handler

import (
	"context"
	"yamdc/model"
	"yamdc/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type tagMappingHandler struct {
	mapper *utils.TagMapper
}

func (h *tagMappingHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	logger := logutil.GetLogger(ctx)

	// 如果映射器未启用，直接返回
	if h.mapper == nil || !h.mapper.IsEnabled() {
		logger.Debug("tag mapper is disabled, skip tag mapping")
		return nil
	}

	// 如果标签列表为空，直接返回
	if len(fc.Meta.Genres) == 0 {
		logger.Debug("no tags to process, skip tag mapping")
		return nil
	}

	// 记录原始标签
	originalTags := fc.Meta.Genres
	logger.Debug("processing tags", zap.Strings("original_tags", originalTags))

	// 处理标签
	processedTags := h.mapper.ProcessTags(originalTags)

	// 更新标签列表
	fc.Meta.Genres = processedTags

	logger.Debug("tag mapping completed",
		zap.Strings("original_tags", originalTags),
		zap.Strings("processed_tags", processedTags))

	return nil
}

// createTagMappingHandler 创建标签映射处理器
func createTagMappingHandler(args interface{}) (IHandler, error) {
	mapper, ok := args.(*utils.TagMapper)
	if !ok {
		// 如果没有配置参数，创建禁用状态的处理器
		disableMapper, _ := utils.NewTagMapper(false, "")
		return &tagMappingHandler{mapper: disableMapper}, nil
	}

	return &tagMappingHandler{mapper: mapper}, nil
}

func init() {
	Register(HTagMapper, createTagMappingHandler)
}
