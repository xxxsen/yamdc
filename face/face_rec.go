package face

import (
	"fmt"
	"image"
)

var defaultInst IFaceRec

func SetFaceRec(impl IFaceRec) {
	defaultInst = impl
}

type IFaceRec interface {
	Name() string
	SearchFaces(data []byte) ([]image.Rectangle, error)
}

func SearchFaces(data []byte) ([]image.Rectangle, error) {
	if defaultInst == nil {
		return nil, fmt.Errorf("not impl")
	}
	return defaultInst.SearchFaces(data)
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
