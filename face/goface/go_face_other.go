//go:build !linux
// +build !linux

package goface

import (
	"errors"
	"yamdc/face"
)

var errFeatureNotSupport = errors.New("feature not support")

func NewGoFace(modelDir string) (face.IFaceRec, error) {
	return nil, errFeatureNotSupport
}
