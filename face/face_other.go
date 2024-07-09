//go:build !linux
// +build !linux

package face

import (
	"errors"
	"image"
)

var errFeatureNotSupport = errors.New("feature not support")

func Init(modelDir string) error {
	return errFeatureNotSupport
}

func IsFaceRecognizeEnabled() bool {
	return false
}

func SearchFaces(data []byte) ([]image.Rectangle, error) {
	return nil, errFeatureNotSupport
}
