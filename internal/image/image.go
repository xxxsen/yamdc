package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // register GIF decoder
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"os"

	_ "golang.org/x/image/bmp" // register BMP decoder
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP decoder
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
		return nil, fmt.Errorf("decode image failed: %w", err)
	}
	return img, nil
}

const defaultJpegQuality = 95

func toJpegData(img image.Image) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{
		Quality: defaultJpegQuality,
	}); err != nil {
		return nil, fmt.Errorf("unable to convert img to jpg, err:%w", err)
	}
	return buf.Bytes(), nil
}

func WriteImageToBytes(img image.Image) ([]byte, error) {
	return toJpegData(img)
}

func fillImage(img *image.RGBA, c color.RGBA) {
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
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
		return nil, fmt.Errorf("encode jpeg failed: %w", err)
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
	if err := os.WriteFile(dst, raw, 0o600); err != nil {
		return fmt.Errorf("write image file: %w", err)
	}
	return nil
}
