package handler

import (
	"context"
	"fmt"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type imageCutter func(data []byte) ([]byte, error)

type posterCropHandler struct {
	faceRec face.IFaceRec
	storage store.IStorage
}

func (c *posterCropHandler) Name() string {
	return HPosterCropper
}

func (c *posterCropHandler) censorCutter(ctx context.Context) imageCutter {
	// 如果没有开启人脸识别, 那么直接使用基础裁剪方案
	if c.faceRec == nil {
		return image.CutCensoredImageFromBytes
	}
	return func(data []byte) ([]byte, error) {
		raw, err := image.CutCensoredImageFromBytes(data)
		if err != nil {
			return nil, fmt.Errorf("cut censored image failed: %w", err)
		}
		// 出错或者已经存在人脸, 直接返回
		rects, err := c.faceRec.SearchFaces(ctx, raw)
		if err != nil || len(rects) > 0 {
			return raw, nil //nolint:nilerr // fallback to basic crop when face search fails
		}
		// 在原始图中, 存在人脸, 那么尝试从原始图中进行人脸识别并裁剪
		// 主要优化部分影片 ID 无法截取正常带人脸 poster 的问题
		rects, err = c.faceRec.SearchFaces(ctx, data)
		if err != nil || len(rects) != 1 {
			return raw, nil //nolint:nilerr // fallback to basic crop when face search fails or multiple faces
		}
		logutil.GetLogger(ctx).Info("enhance poster crop with face rec for censored number")
		rec, err := image.CutImageWithFaceRecFromBytesWithFaceRec(ctx, c.faceRec, data)
		if err != nil {
			return raw, nil //nolint:nilerr // fallback to basic crop when face-rec crop fails
		}
		return rec, nil
	}
}

func (c *posterCropHandler) unratedCutter(ctx context.Context) imageCutter {
	if c.faceRec == nil {
		return image.CutCensoredImageFromBytes
	}
	return func(data []byte) ([]byte, error) {
		rec, err := image.CutImageWithFaceRecFromBytesWithFaceRec(ctx, c.faceRec, data)
		if err == nil {
			return rec, nil
		}
		logutil.GetLogger(ctx).Warn("cut image with face rec failed, try use other cut method", zap.Error(err))
		return image.CutCensoredImageFromBytes(data)
	}
}

func (c *posterCropHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	logger := logutil.GetLogger(ctx).With(zap.String("number", fc.Meta.Number))
	if fc.Meta.Poster != nil { // 仅处理没有海报的元数据
		logger.Debug("poster exist, skip generate")
		return nil
	}
	if fc.Meta.Cover == nil { // 无封面, 处理无意义
		logger.Error("no cover found, skip process poster")
		return nil
	}
	cutter := c.censorCutter(ctx)            // 默认情况下使用基础封面裁剪
	if fc.Number.GetExternalFieldUnrated() { // 带有附加标记时优先尝试人脸识别方案
		cutter = c.unratedCutter(ctx)
	}
	raw, err := c.storage.GetData(ctx, fc.Meta.Cover.Key)
	if err != nil {
		return fmt.Errorf("read cover data failed, err:%w", err)
	}
	nextRaw, err := cutter(raw)
	if err != nil {
		return fmt.Errorf("save cutted poster data failed, err:%w", err)
	}
	key, err := store.AnonymousPutDataTo(ctx, c.storage, nextRaw)
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
	Register(HPosterCropper, func(_ any, deps appdeps.Runtime) (IHandler, error) {
		return &posterCropHandler{
			faceRec: deps.FaceRec,
			storage: deps.Storage,
		}, nil
	})
}
