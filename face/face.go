package face

import (
	"fmt"
	"sync"

	"github.com/Kagami/go-face"
)

var once sync.Once
var recInst *face.Recognizer

func Init(modelDir string) error {
	var err error
	once.Do(func() {
		recInst, err = face.NewRecognizer(modelDir)
	})
	if err != nil {
		return err
	}
	return nil
}

func getRecInst() (*face.Recognizer, bool) {
	if recInst == nil {
		return nil, false
	}
	return recInst, true
}

func SearchFaces(data []byte) ([]face.Face, error) {
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
	return fces, nil
}

func FindMaxFace(fs []face.Face) face.Face {
	var maxArea int
	var m face.Face
	for _, f := range fs {
		p := f.Rectangle.Size()
		if area := p.X * p.Y; area > maxArea {
			m = f
			maxArea = area
		}
	}
	return m
}
