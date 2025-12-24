package image

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmallWatermark(t *testing.T) {
	watermark := MakeColorImage(image.Rect(0, 0, 768, 374), color.RGBA{255, 0, 0, 0})
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "watermark.jpeg"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	assert.NoError(t, err)
	defer f.Close()
	err = jpeg.Encode(f, watermark, nil)
	assert.NoError(t, err)
}

func TestWatermark(t *testing.T) {
	frame := MakeColorImage(image.Rect(0, 0, 380, 540), color.RGBA{0, 0, 0, 255})
	wms := make([]image.Image, 0, 5)
	for i := 0; i < 4; i++ {
		watermark := MakeColorImage(image.Rect(0, 0, 768, 374), color.RGBA{255, 0, 0, 0})
		wms = append(wms, watermark)
	}
	img, err := addWatermarkToImage(frame, wms)
	assert.NoError(t, err)
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "fill_watermark.jpeg"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	assert.NoError(t, err)
	defer f.Close()
	err = jpeg.Encode(f, img, nil)
	assert.NoError(t, err)
}

func TestWatermarkWithRes(t *testing.T) {
	data, err := MakeColorImageData(image.Rect(0, 0, 380, 540), color.RGBA{0, 0, 0, 255})
	assert.NoError(t, err)
	raw, err := AddWatermarkFromBytes(data, []Watermark{
		WMChineseSubtitle,
		WMUncensored,
		WM4K,
	})
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(os.TempDir(), "fill_watermark_with_res.jpeg"), raw, 0644)
	assert.NoError(t, err)
}
