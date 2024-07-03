package handler

import (
	"av-capture/image"
	"av-capture/model"
	"av-capture/store"
	"context"
	"fmt"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type imageCutter func(data []byte) ([]byte, error)

type posterCropper struct {
}

func (c *posterCropper) Name() string {
	return HPosterCropper
}

func (c *posterCropper) wrapCutImageWithFaceRec(ctx context.Context, fallback imageCutter) imageCutter {
	return func(data []byte) ([]byte, error) {
		data, err := image.CutImageWithFaceRec(data)
		if err == nil {
			return data, nil
		}
		logutil.GetLogger(ctx).Warn("cut image with face rec failed, try use other cut method", zap.Error(err))
		return fallback(data)
	}
}

func (c *posterCropper) Handle(ctx context.Context, fc *model.FileContext) error {
	logger := logutil.GetLogger(ctx).With(zap.String("number", fc.Meta.Number))
	if fc.Meta.Poster != nil { //仅处理没有海报的元数据
		logger.Debug("poster exist, skip generate")
		return nil
	}
	if fc.Meta.Cover == nil { //无封面, 处理无意义
		logger.Error("no cover found, skip process poster")
		return nil
	}
	var cutter imageCutter = image.CutCensoredImage //默认情况下, 都按骑兵进行封面处理
	if fc.Number.IsUncensorMovie() {                //如果为步兵, 则使用人脸识别
		cutter = c.wrapCutImageWithFaceRec(ctx, image.CutCensoredImage)
	}
	data, err := store.GetDefault().GetData(fc.Meta.Cover.Key)
	if err != nil {
		return fmt.Errorf("get cover data failed, err:%w, key:%s", err, fc.Meta.Cover.Key)
	}
	res, err := cutter(data)
	if err != nil {
		return fmt.Errorf("cut poster image failed, err:%w", err)
	}
	key, err := store.GetDefault().Put(res)
	if err != nil {
		return fmt.Errorf("save cutted poster data failed, err:%w", err)
	}
	//TODO: 如果实在无法裁剪出有效的封面, 那么尝试取一张竖屏的, 或者直接使用骑兵的裁剪逻辑
	fc.Meta.Poster = &model.File{
		Name: "./poster.jpg",
		Key:  key,
	}
	return nil
}

func init() {
	Register(HPosterCropper, HandlerToCreator(&posterCropper{}))
}
