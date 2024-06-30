package processor

import (
	"av-capture/image"
	"av-capture/model"
	"context"
	"fmt"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type posterProcessor struct {
}

func createPosterProcessor(args interface{}) (IProcessor, error) {
	return &posterProcessor{}, nil
}

func (c *posterProcessor) Name() string {
	return PsPosterCropper
}

func (c *posterProcessor) Process(ctx context.Context, fc *model.FileContext) error {
	logger := logutil.GetLogger(ctx).With(zap.String("number", fc.Meta.Number))
	if fc.Meta.Poster != nil { //仅处理没有海报的元数据
		logger.Debug("poster exist, skip generate")
		return nil
	}
	if fc.Meta.Cover == nil { //无封面, 处理无意义
		logger.Error("no cover found, skip process poster")
		return nil
	}
	cutter := image.CutCensoredImage   //默认情况下, 都按骑兵进行封面处理
	if fc.NumberInfo.IsUncensorMovie { //如果为步兵, 则使用人脸识别
		cutter = image.CutImageWithFaceRec
	}
	res, err := cutter(fc.Meta.Cover.Data)
	if err != nil {
		return fmt.Errorf("cut poster image failed, err:%w", err)
	}
	fc.Meta.Poster = &model.Image{
		Name: "./poster.jpg",
		Data: res,
	}
	return nil
}

func (c *posterProcessor) IsOptional() bool {
	return false
}

func init() {
	Register(PsPosterCropper, createPosterProcessor)
}
