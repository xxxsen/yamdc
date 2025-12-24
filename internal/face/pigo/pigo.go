package pigo

import (
	"bytes"
	"context"
	"image"
	"os"
	"path/filepath"
	"github.com/xxxsen/yamdc/internal/face"

	pigo "github.com/esimov/pigo/core"
)

const (
	defaultFaceFinderCascade = "facefinder"
)

type pigoWrap struct {
	inst *pigo.Pigo
}

func NewPigo(models string) (face.IFaceRec, error) {
	csFileData, err := os.ReadFile(filepath.Join(models, defaultFaceFinderCascade))
	if err != nil {
		return nil, err
	}
	pigo := pigo.NewPigo()
	classifier, err := pigo.Unpack(csFileData)
	if err != nil {
		return nil, err
	}
	return &pigoWrap{inst: classifier}, nil
}

func (w *pigoWrap) Name() string {
	return face.NamePigo
}

func (w *pigoWrap) SearchFaces(ctx context.Context, data []byte) ([]image.Rectangle, error) {
	img, err := pigo.DecodeImage(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	pixels := pigo.RgbToGrayscale(img)
	cols, rows := img.Bounds().Max.X, img.Bounds().Max.Y
	cParams := pigo.CascadeParams{
		MinSize:     20,
		MaxSize:     1000,
		ShiftFactor: 0.1,
		ScaleFactor: 1.1,

		ImageParams: pigo.ImageParams{
			Pixels: pixels,
			Rows:   rows,
			Cols:   cols,
			Dim:    cols,
		},
	}
	angle := 0.0
	dets := w.inst.RunCascade(cParams, angle)
	dets = w.inst.ClusterDetections(dets, 0.2)
	rs := make([]image.Rectangle, 0, len(dets))
	for _, det := range dets {
		if det.Q < 0.5 {
			continue
		}
		x1 := det.Col - det.Scale/2
		y1 := det.Row - det.Scale/2
		x2 := det.Col + det.Scale/2
		y2 := det.Row + det.Scale/2
		rs = append(rs, image.Rect(x1, y1, x2, y2))
	}
	return rs, nil
}
