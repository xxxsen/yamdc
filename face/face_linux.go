//go:build linux
// +build linux

package face

import (
	"fmt"
	"image"

	"github.com/Kagami/go-face"
)

var recInst *face.Recognizer

func Init(modelDir string) error {
	inst, err := face.NewRecognizer(modelDir)
	if err != nil {
		return err
	}
	recInst = inst
	return nil
}

func SearchFaces(data []byte) ([]image.Rectangle, error) {
	inst, ok := getRecInst()
	if !ok {
		return nil, fmt.Errorf("face rec inst not init")
	}
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

func IsFaceRecognizeEnabled() bool {
	return recInst != nil
}

func getRecInst() (*face.Recognizer, bool) {
	if recInst == nil {
		return nil, false
	}
	return recInst, true
}
