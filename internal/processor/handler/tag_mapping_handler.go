package handler

import (
	"context"
	"os"
	"github.com/xxxsen/yamdc/internal/model"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/common/utils"
	"go.uber.org/zap"
)

type tagMappingHandler struct {
	mapper *TagMapper
}

type tagMappingConfig struct {
	FilePath string `json:"file_path"`
}

func (h *tagMappingHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	logger := logutil.GetLogger(ctx)

	// 如果映射器未启用，直接返回
	if h.mapper == nil {
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
	c := &tagMappingConfig{}

	handler := &tagMappingHandler{}

	if err := utils.ConvStructJson(args, c); err != nil {
		return nil, err
	}
	// 如果映射器未启用，直接返回
	if c.FilePath == "" {
		return handler, nil
	}
	// 默认所有功能都是开启的(disable=false)
	// 为了兼容旧版本配置文件, 如果配置文件不存在, 也不开启这个功能
	if _, err := os.Stat(c.FilePath); os.IsNotExist(err) {
		return handler, nil
	}

	mapper, err := NewTagMapper(c.FilePath)
	if err != nil {
		return nil, err
	}
	handler.mapper = mapper
	return handler, nil
}

func init() {
	Register(HTagMapper, createTagMappingHandler)
}
