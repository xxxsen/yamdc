package handler

import (
	"context"
	"fmt"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type imageCutter func(data []byte) ([]byte, error)

type posterCropHandler struct {
}

func (c *posterCropHandler) Name() string {
	return HPosterCropper
}

func (c *posterCropHandler) censorCutter(ctx context.Context) imageCutter {
	//如果没有开启人脸识别, 那么直接使用原始的骑兵裁剪方案
	if !face.IsFaceRecognizeEnabled() {
		return image.CutCensoredImageFromBytes
	}
	return func(data []byte) ([]byte, error) {
		raw, err := image.CutCensoredImageFromBytes(data)
		if err != nil {
			return nil, err
		}
		//出错或者已经存在人脸, 直接返回
		rects, err := face.SearchFaces(ctx, raw)
		if err != nil || len(rects) > 0 {
			return raw, nil
		}
		//在原始图中, 存在人脸, 那么尝试从原始图中进行人脸识别并裁剪
		//主要优化 SIRO 之类的番号无法截取正常带人脸poster的问题
		rects, err = face.SearchFaces(ctx, data)
		if err != nil || len(rects) != 1 { //仅有一个人脸的场景下才执行人脸识别, 避免截到奇奇怪怪的地方
			return raw, nil
		}
		logutil.GetLogger(ctx).Info("enhance poster crop with face rec for censored number")
		rec, err := image.CutImageWithFaceRecFromBytes(ctx, data)
		if err != nil {
			return raw, nil
		}
		return rec, nil
	}
}

func (c *posterCropHandler) uncensorCutter(ctx context.Context) imageCutter {
	if !face.IsFaceRecognizeEnabled() {
		return image.CutCensoredImageFromBytes
	}
	return func(data []byte) ([]byte, error) {
		rec, err := image.CutImageWithFaceRecFromBytes(ctx, data)
		if err == nil {
			return rec, nil
		}
		logutil.GetLogger(ctx).Warn("cut image with face rec failed, try use other cut method", zap.Error(err))
		return image.CutCensoredImageFromBytes(data)
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
	var cutter imageCutter = c.censorCutter(ctx) //默认情况下, 都按骑兵进行封面处理
	if fc.Number.GetExternalFieldUncensor() {    //如果为步兵, 则使用人脸识别(当然, 只有该特性能用的情况下才启用)
		cutter = c.uncensorCutter(ctx)
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
