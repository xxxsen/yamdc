package handler

import (
	"context"
	"fmt"
	"yamdc/face"
	"yamdc/image"
	"yamdc/model"
	"yamdc/store"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type imageCutter func(data []byte) ([]byte, error)

type posterCropHandler struct {
}

func (c *posterCropHandler) Name() string {
	return HPosterCropper
}

func (c *posterCropHandler) wrapCutImageWithFaceRec(ctx context.Context, fallback imageCutter) imageCutter {
	return func(data []byte) ([]byte, error) {
		rec, err := image.CutImageWithFaceRecFromBytes(ctx, data)
		if err == nil {
			return rec, nil
		}
		logutil.GetLogger(ctx).Warn("cut image with face rec failed, try use other cut method", zap.Error(err))
		return fallback(data)
	}
}

func (c *posterCropHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	logger := logutil.GetLogger(ctx).With(zap.String("number", fc.Meta.Number))
	if fc.Meta.Poster != nil { //仅处理没有海报的元数据
		logger.Debug("poster exist, skip generate")
		return nil
	}
	if fc.Meta.Cover == nil { //无封面, 处理无意义
		logger.Error("no cover found, skip process poster")
		return nil
	}
	var cutter imageCutter = image.CutCensoredImageFromBytes                   //默认情况下, 都按骑兵进行封面处理
	if fc.Number.GetExternalFieldUncensor() && face.IsFaceRecognizeEnabled() { //如果为步兵, 则使用人脸识别(当然, 只有该特性能用的情况下才启用)
		cutter = c.wrapCutImageWithFaceRec(ctx, image.CutCensoredImageFromBytes)
	}
	key, err := store.AnonymousDataRewrite(ctx, fc.Meta.Cover.Key, func(ctx context.Context, data []byte) ([]byte, error) {
		return cutter(data)
	})
	if err != nil {
		return fmt.Errorf("save cutted poster data failed, err:%w", err)
	}
	fc.Meta.Poster = &model.File{
		Name: "./poster.jpg",
		Key:  key,
	}
	return nil
}

func init() {
	Register(HPosterCropper, HandlerToCreator(&posterCropHandler{}))
}
