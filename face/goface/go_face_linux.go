//go:build linux
// +build linux

package goface

import (
	"image"
	"yamdc/face"

	goface "github.com/Kagami/go-face"
)

type goFace struct {
	recInst *goface.Recognizer
}

func (f *goFace) SearchFaces(data []byte) ([]image.Rectangle, error) {
	inst := f.recInst
	fces, err := inst.RecognizeCNN(data)
	if err != nil {
		return nil, err
	}
	if len(fces) == 0 {
		fces, err = inst.Recognize(data)
	}
	if err != nil {
		return nil, err
	}
	rs := make([]image.Rectangle, 0, len(fces))
	for _, fce := range fces {
		rs = append(rs, fce.Rectangle)
	}
	return rs, nil
}

func (f *goFace) Name() string {
	return face.NameGoFace
}

func NewGoFace(modelDir string) (face.IFaceRec, error) {
	inst, err := goface.NewRecognizer(modelDir)
	if err != nil {
		return nil, err
	}
	return &goFace{recInst: inst}, nil
}
