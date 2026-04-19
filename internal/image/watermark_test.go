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
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "watermark.jpeg"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	assert.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()
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
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "fill_watermark.jpeg"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	assert.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()
	err = jpeg.Encode(f, img, nil)
	assert.NoError(t, err)
}

func TestWatermarkWithRes(t *testing.T) {
	data, err := MakeColorImageData(image.Rect(0, 0, 380, 540), color.RGBA{0, 0, 0, 255})
	assert.NoError(t, err)
	raw, err := AddWatermarkFromBytes(data, []Watermark{
		WMChineseSubtitle,
		WMUnrated,
		WM4K,
	})
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(os.TempDir(), "fill_watermark_with_res.jpeg"), raw, 0o600) //nolint:gosec // test file in temp dir
	assert.NoError(t, err)
}

func TestAddWatermarkToImage_errors(t *testing.T) {
	t.Parallel()
	frame := MakeColorImage(image.Rect(0, 0, 200, 80), color.RGBA{A: 255})
	wm := MakeColorImage(image.Rect(0, 0, 64, 64), color.RGBA{R: 255, A: 255})

	_, err := addWatermarkToImage(frame, nil)
	assert.ErrorIs(t, err, errNoWatermarkFound)

	seven := make([]image.Image, 7)
	for i := range seven {
		seven[i] = wm
	}
	_, err = addWatermarkToImage(frame, seven)
	assert.Error(t, err)

	sixTall := make([]image.Image, 6)
	for i := range sixTall {
		sixTall[i] = wm
	}
	short := MakeColorImage(image.Rect(0, 0, 380, 100), color.RGBA{A: 255})
	_, err = addWatermarkToImage(short, sixTall)
	assert.ErrorIs(t, err, errImageHeightTooSmall)
}

func TestAddWatermark_errors(t *testing.T) {
	t.Parallel()
	frame := MakeColorImage(image.Rect(0, 0, 200, 200), color.RGBA{A: 255})

	_, err := AddWatermark(frame, nil)
	assert.ErrorIs(t, err, errNoWatermarkFound)

	_, err = AddWatermark(frame, []Watermark{Watermark(999)})
	assert.ErrorIs(t, err, errWatermarkNotFound)
}

func TestAddWatermarkFromBytes_loadError(t *testing.T) {
	t.Parallel()
	_, err := AddWatermarkFromBytes([]byte("not jpeg"), []Watermark{WM4K})
	assert.Error(t, err)
}

func TestSelectWatermarkResource_unknown(t *testing.T) {
	t.Parallel()
	_, ok := selectWatermarkResource(Watermark(0))
	assert.False(t, ok)
}

func TestAddWatermark_canvasTooShortForStack(t *testing.T) {
	t.Parallel()
	short := MakeColorImage(image.Rect(0, 0, 400, 80), color.RGBA{A: 255})
	_, err := AddWatermark(short, []Watermark{
		WMChineseSubtitle, WMUnrated, WM4K, WMSpecialEdition, WM8K, WMVR,
	})
	assert.ErrorIs(t, err, errImageHeightTooSmall)
}
