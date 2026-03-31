package face

import (
	"context"
	"image"
)

type IFaceRec interface {
	Name() string
	SearchFaces(ctx context.Context, data []byte) ([]image.Rectangle, error)
}

func FindMaxFace(fs []image.Rectangle) image.Rectangle {
	var maxArea int
	var m image.Rectangle
	for _, f := range fs {
		p := f.Size()
		if area := p.X * p.Y; area > maxArea {
			m = f
			maxArea = area
		}
	}
	return m
}
