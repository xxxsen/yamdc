package image

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"yamdc/image/resource"
)

type Watermark int

const (
	WMChineseSubtitle Watermark = 1
	WMUncensored      Watermark = 2
	WM4K              Watermark = 3
)

const (
	defaultMaxWaterMarkCount               = 4                    //最大的水印个数
	defaultWaterMarkWidthToImageWidthRatio = float64(31.58) / 100 //水印与整张图片的宽度比, W(watermark)/W(image) = 0.3158
	defaultWaterMarkWithToHeightRatio      = 2                    //水印本身的宽高比, W(watermark)/H(watermark) = 2
	defaultWatermarkGapSize                = 10                   //2个水印之间的间隔
)

func addWatermarkToImage(img image.Image, wms []image.Image) (image.Image, error) {
	if len(wms) > defaultMaxWaterMarkCount {
		return nil, fmt.Errorf("water mark count out of limit, size:%d", len(wms))
	}
	if len(wms) == 0 {
		return nil, fmt.Errorf("no watermark found")
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
			return nil, fmt.Errorf("image height too smart to contains all watermark")
		}
		draw.Draw(newImg, rect, wm, image.Point{0, 0}, draw.Over)
	}
	return newImg, nil
}

func selectWatermarkResource(w Watermark) ([]byte, bool) {
	var out []byte
	switch w {
	case WMChineseSubtitle:
		out = resource.ResIMGSubtitle
	case WMUncensored:
		out = resource.ResIMGUncensored
	case WM4K:
		out = resource.ResIMG4K
	default:
		break
	}
	if len(out) == 0 {
		return nil, false
	}
	rs := make([]byte, len(out))
	copy(rs, out)
	return rs, true
}

func AddWatermark(img image.Image, wmTags []Watermark) (image.Image, error) {
	wms := make([]image.Image, 0, len(wmTags))
	for _, tag := range wmTags {
		res, ok := selectWatermarkResource(tag)
		if !ok {
			return nil, fmt.Errorf("watermark:%d not found", tag)
		}
		wm, _, err := image.Decode(bytes.NewReader(res))
		if err != nil {
			return nil, err
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
