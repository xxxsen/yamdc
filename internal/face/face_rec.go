package face

import (
	"context"
	"fmt"
	"image"
)

var defaultInst IFaceRec

func SetFaceRec(impl IFaceRec) {
	defaultInst = impl
}

type IFaceRec interface {
	Name() string
	SearchFaces(ctx context.Context, data []byte) ([]image.Rectangle, error)
}

func SearchFaces(ctx context.Context, data []byte) ([]image.Rectangle, error) {
	if defaultInst == nil {
		return nil, fmt.Errorf("not impl")
	}
	return defaultInst.SearchFaces(ctx, data)
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

func IsFaceRecognizeEnabled() bool {
	return defaultInst != nil
}
