package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

func TranscodeToJpeg(data []byte) ([]byte, error) {
	img, err := LoadImage(data)
	if err != nil {
		return nil, err
	}
	return toJpegData(img)
}

func LoadImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func toJpegData(img image.Image) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{
		Quality: 100,
	}); err != nil {
		return nil, fmt.Errorf("unable to convert img to jpg, err:%w", err)
	}
	return buf.Bytes(), nil
}

func WriteImageToBytes(img image.Image) ([]byte, error) {
	return toJpegData(img)
}

func fillImage(img *image.RGBA, c color.RGBA) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, c)
		}
	}
}

func MakeColorImage(rect image.Rectangle, rgb color.RGBA) image.Image {
	img := image.NewRGBA(rect)
	fillImage(img, rgb)
	return img
}

func MakeColorImageData(rect image.Rectangle, rgb color.RGBA) ([]byte, error) {
	img := MakeColorImage(rect, rgb)
	buf := bytes.Buffer{}
	err := jpeg.Encode(&buf, img, nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Scale(src image.Image, frame image.Rectangle) image.Image {
	dst := image.NewRGBA(frame)
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

func WriteImageToFile(dst string, img image.Image) error {
	raw, err := WriteImageToBytes(img)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0644)
}
