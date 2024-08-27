package handler

import (
	"context"
	"fmt"
	"strings"
	"yamdc/ffmpeg"
	"yamdc/image"
	"yamdc/model"
	"yamdc/store"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type imageTranscodeHandler struct {
}

func (p *imageTranscodeHandler) Name() string {
	return HImageTranscoder
}

func (p *imageTranscodeHandler) Handle(ctx context.Context, meta *model.FileContext) error {
	meta.Meta.Cover = p.transcode(ctx, "cover", meta.Meta.Cover)
	meta.Meta.Poster = p.transcode(ctx, "poster", meta.Meta.Poster)
	rebuildSampleList := make([]*model.File, 0, len(meta.Meta.SampleImages))
	for idx, item := range meta.Meta.SampleImages {
		if v := p.transcode(ctx, fmt.Sprintf("sample_%d", idx), item); v != nil {
			rebuildSampleList = append(rebuildSampleList, v)
		}
	}
	meta.Meta.SampleImages = rebuildSampleList
	return nil
}

func (p *imageTranscodeHandler) transcode(ctx context.Context, name string, f *model.File) *model.File {
	logger := logutil.GetLogger(ctx).With(zap.String("name", name))
	if f == nil || len(f.Key) == 0 {
		logger.Debug("no image found, skip transcode to jpeg logic")
		return nil
	}
	logger = logger.With(zap.String("key", f.Key))
	data, err := store.GetData(ctx, f.Key)
	if err != nil {
		logger.Debug("read key data failed", zap.Error(err))
		return f //不丢弃, 后续处理的时候报错, 方便发现问题
	}
	raw, err := image.TranscodeToJpeg(data)
	if err != nil && strings.Contains(err.Error(), "luma/chroma subsampling ratio") && ffmpeg.IsFFMpegEnabled() {
		data, err = ffmpeg.ConvertToYuv420pJpegFromBytes(ctx, data)
		if err != nil {
			logger.Error("use ffmpeg to correct invalid image data failed", zap.Error(err))
			return nil
		}
		raw, err = image.TranscodeToJpeg(data)
	}
	if err != nil {
		logger.Error("unable to convert image to jpeg format", zap.Error(err))
		return nil
	}
	key, err := store.AnonymousPutData(ctx, raw)
	if err != nil {
		logger.Error("store transcoded image data failed", zap.Error(err))
		return f //
	}
	logger.Debug("transcode image succ", zap.String("new_key", key))
	f.Key = key
	return f
}

func init() {
	Register(HImageTranscoder, HandlerToCreator(&imageTranscodeHandler{}))
}
