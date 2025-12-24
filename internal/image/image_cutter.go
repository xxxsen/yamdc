package image

import (
	"context"
	"fmt"
	"image"
	"math"
	"github.com/xxxsen/yamdc/internal/face"
)

const (
	defaultAspectRatio = 2.0 / 3.0 //poster的宽高比实际为2:3
)

func determineCutFrameViaHeight(dx, dy int, dxCenter int, aspectRatio float64) (image.Rectangle, error) {
	cropWidth := int(float64(dy) * aspectRatio)
	cropWidthLeft := dxCenter - cropWidth/2
	cropWidthRight := dxCenter + cropWidth/2
	if cropWidthLeft < 0 {
		cropWidthRight += int(math.Abs(float64(cropWidthLeft)))
		cropWidthLeft = 0
	}
	if cropWidthRight > dx {
		cropWidthLeft -= cropWidthRight - dx
		cropWidthRight = dx
	}
	if cropWidthLeft < 0 || cropWidthRight > dx {
		return image.Rectangle{}, fmt.Errorf("unable to crop satisfy image, left:%d, right:%d", cropWidthLeft, cropWidthRight)
	}
	return image.Rectangle{
		Min: image.Point{cropWidthLeft, 0},
		Max: image.Point{cropWidthRight, dy},
	}, nil
}

func determineCutFrameViaWidth(dx, dy int, dyCenter int, aspectRatio float64) (image.Rectangle, error) {
	cropHeight := int(float64(dx) / aspectRatio)
	cropHeightTop := dyCenter - cropHeight/2
	cropHeightBottom := dyCenter + cropHeight/2
	if cropHeightTop < 0 {
		cropHeightBottom += int(math.Abs(float64(cropHeightTop)))
		cropHeightTop = 0
	}
	if cropHeightBottom > dy {
		cropHeightTop -= cropHeightBottom - dy
		cropHeightBottom = dy
	}
	if cropHeightTop < 0 || cropHeightBottom > dy {
		return image.Rectangle{}, fmt.Errorf("unable to crop satisfy image, top:%d, bottom:%d", cropHeightTop, cropHeightBottom)
	}
	return image.Rectangle{
		Min: image.Point{0, cropHeightTop},
		Max: image.Point{dx, cropHeightBottom},
	}, nil
}

// DetermineCutFrame 根据图片宽高及截取中心点, 计算出最终截图的边框
func DetermineCutFrame(dx, dy int, dxCenter, dyCenter int, aspectRatio float64) (image.Rectangle, error) {
	if dx == 0 || dy == 0 {
		return image.Rectangle{}, fmt.Errorf("invalid image resolution")
	}
	if aspectRatio == 0 {
		return image.Rectangle{}, fmt.Errorf("invalid aspectRatio")
	}
	if float64(dx)/float64(dy) > aspectRatio { //宽高比大于预期, 那么可以以高度来反算宽度
		return determineCutFrameViaHeight(dx, dy, dxCenter, aspectRatio)
	}
	return determineCutFrameViaWidth(dx, dy, dyCenter, aspectRatio)
}

func CutImageViaRectangle(img image.Image, rect image.Rectangle) (image.Image, error) {
	if img.Bounds().Max.X < rect.Max.X || img.Bounds().Max.Y < rect.Max.Y {
		return nil, fmt.Errorf("invalid rectangle")
	}
	croppedImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)
	return croppedImg, nil
}

func CutCensoredImage(img image.Image) (image.Image, error) {
	//从最右侧进行裁剪
	cutFrame, err := DetermineCutFrame(img.Bounds().Dx(), img.Bounds().Dy(), img.Bounds().Dx(), 0, defaultAspectRatio)
	if err != nil {
		return nil, fmt.Errorf("unable to determine cut frame, err:%w", err)
	}
	return CutImageViaRectangle(img, cutFrame)
}

func CutImageWithFaceRec(ctx context.Context, img image.Image) (image.Image, error) {
	data, err := toJpegData(img)
	if err != nil {
		return nil, err
	}
	fs, err := face.SearchFaces(ctx, data)
	if err != nil {
		return nil, err
	}
	if len(fs) == 0 {
		return nil, fmt.Errorf("no face found")
	}
	selectedFace := face.FindMaxFace(fs)

	cutFrame, err := DetermineCutFrame(
		img.Bounds().Dx(), img.Bounds().Dy(),
		selectedFace.Min.X+selectedFace.Dx()/2, selectedFace.Min.Y+selectedFace.Dy()/2,
		defaultAspectRatio)
	if err != nil {
		return nil, fmt.Errorf("unable to determine cut frame, err:%w", err)
	}
	return CutImageViaRectangle(img, cutFrame)
}

func CutImageWithFaceRecFromBytes(ctx context.Context, data []byte) ([]byte, error) {
	img, err := LoadImage(data)
	if err != nil {
		return nil, err
	}
	cutted, err := CutImageWithFaceRec(ctx, img)
	if err != nil {
		return nil, err
	}
	return WriteImageToBytes(cutted)
}

func CutCensoredImageFromBytes(data []byte) ([]byte, error) {
	img, err := LoadImage(data)
	if err != nil {
		return nil, err
	}
	newImg, err := CutCensoredImage(img)
	if err != nil {
		return nil, err
	}
	return WriteImageToBytes(newImg)
}
