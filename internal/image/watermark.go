package image

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"

	"github.com/xxxsen/yamdc/internal/resource"
)

var (
	errWatermarkCountLimit = errors.New("water mark count out of limit")
	errNoWatermarkFound    = errors.New("no watermark found")
	errImageHeightTooSmall = errors.New("image height too small to contain all watermark")
	errWatermarkNotFound   = errors.New("watermark not found")
)

// Watermark 枚举每一种可画在封面上的水印类型.
//
// 消费方: internal/processor/handler/watermark_handler.go 按
// MovieMeta.Genres 里的 tag 决定应该打哪些 Watermark, 匹配表见
// 该 handler 中的 defaultWatermarkRules. 新增一种水印时:
//  1. 在此处追加枚举值;
//  2. 在 registerResource 里挂上图像资源;
//  3. 在 watermark_handler 的 rule 表中加一行 tag -> Watermark 映射.
type Watermark int

const (
	WMChineseSubtitle Watermark = 1
	WMUnrated         Watermark = 2
	WM4K              Watermark = 3
	WMSpecialEdition  Watermark = 4
	WM8K              Watermark = 5
	WMVR              Watermark = 6
	WMRestored        Watermark = 7
)

var imgCache = make(map[Watermark]image.Image)

func decodeEmbeddedImage(data []byte) image.Image {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("decode embedded watermark resource failed: %v", err))
	}
	return img
}

func registerResource() {
	imgCache[WMChineseSubtitle] = decodeEmbeddedImage(resource.ResIMGSubtitle)
	imgCache[WM4K] = decodeEmbeddedImage(resource.ResIMG4K)
	imgCache[WMUnrated] = decodeEmbeddedImage(resource.ResIMGUnrated)
	imgCache[WMSpecialEdition] = decodeEmbeddedImage(resource.ResIMGSpecialEdition)
	imgCache[WM8K] = decodeEmbeddedImage(resource.ResIMG8K)
	imgCache[WMVR] = decodeEmbeddedImage(resource.ResIMGVR)
	imgCache[WMRestored] = decodeEmbeddedImage(resource.ResIMGRestored)
}

func init() {
	registerResource()
}

const (
	defaultMaxWaterMarkCount               = 6                    // 最大的水印个数
	defaultWaterMarkWidthToImageWidthRatio = float64(31.58) / 100 // 水印与整张图片的宽度比, W(watermark)/W(image) = 0.3158
	defaultWaterMarkWithToHeightRatio      = 2                    // 水印本身的宽高比, W(watermark)/H(watermark) = 2
	defaultWatermarkGapSize                = 10                   // 2个水印之间的间隔
)

func addWatermarkToImage(img image.Image, wms []image.Image) (image.Image, error) {
	if len(wms) > defaultMaxWaterMarkCount {
		return nil, fmt.Errorf("water mark count out of limit, size:%d: %w", len(wms), errWatermarkCountLimit)
	}
	if len(wms) == 0 {
		return nil, errNoWatermarkFound
	}
	mainBounds := img.Bounds()
	newImg := image.NewRGBA(mainBounds)
	draw.Draw(newImg, mainBounds, img, image.Point{0, 0}, draw.Src)
	watermarkWidth := int(float64(img.Bounds().Dx()) * defaultWaterMarkWidthToImageWidthRatio)
	watermarkHeight := watermarkWidth / 2
	for i := 0; i < len(wms); i++ {
		wm := Scale(wms[len(wms)-i-1], image.Rect(0, 0, watermarkWidth, watermarkHeight))
		rect := image.Rectangle{
			Min: image.Point{
				X: img.Bounds().Dx() - watermarkWidth,
				Y: img.Bounds().Dy() - (i+1)*watermarkHeight - i*defaultWatermarkGapSize,
			},
			Max: image.Point{
				X: img.Bounds().Dx(),
				Y: img.Bounds().Dy() - i*watermarkHeight - i*defaultWatermarkGapSize,
			},
		}
		if rect.Min.Y < 0 || rect.Max.Y < 0 {
			return nil, errImageHeightTooSmall
		}
		draw.Draw(newImg, rect, wm, image.Point{0, 0}, draw.Over)
	}
	return newImg, nil
}

func AddWatermark(img image.Image, wmTags []Watermark) (image.Image, error) {
	wms := make([]image.Image, 0, len(wmTags))
	for _, tag := range wmTags {
		wm, ok := imgCache[tag]
		if !ok {
			return nil, fmt.Errorf("watermark:%d: %w", tag, errWatermarkNotFound)
		}
		wms = append(wms, wm)
	}
	output, err := addWatermarkToImage(img, wms)
	if err != nil {
		return nil, fmt.Errorf("add water mark failed, err:%w", err)
	}
	return output, nil
}

func AddWatermarkFromBytes(data []byte, wmTags []Watermark) ([]byte, error) {
	img, err := LoadImage(data)
	if err != nil {
		return nil, err
	}
	newImg, err := AddWatermark(img, wmTags)
	if err != nil {
		return nil, err
	}
	return WriteImageToBytes(newImg)
}
