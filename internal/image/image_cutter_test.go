package image

import (
	"context"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"yamdc/internal/face"
	"yamdc/internal/face/pigo"

	"github.com/stretchr/testify/assert"
)

type testPair struct {
	//input
	dx, dy             int
	dxCenter, dyCenter int
	//output
	rect   image.Rectangle
	gotErr bool
}

func TestCutFrame(t *testing.T) {
	tests := []testPair{
		//宽高比>70%的场景, 此时会使用高度反推宽度
		{dx: 71, dy: 100, dxCenter: 71, dyCenter: 0, rect: image.Rectangle{Min: image.Point{1, 0}, Max: image.Point{71, 100}}, gotErr: false},
		{dx: 100, dy: 100, dxCenter: 100, dyCenter: 0, rect: image.Rectangle{Min: image.Point{30, 0}, Max: image.Point{100, 100}}, gotErr: false},
		{dx: 100, dy: 100, dxCenter: 50, dyCenter: 0, rect: image.Rectangle{Min: image.Point{15, 0}, Max: image.Point{85, 100}}, gotErr: false},
		{dx: 100, dy: 100, dxCenter: 0, dyCenter: 0, rect: image.Rectangle{Min: image.Point{0, 0}, Max: image.Point{70, 100}}, gotErr: false},
		{dx: 1000, dy: 100, dxCenter: 1000, dyCenter: 0, rect: image.Rectangle{Min: image.Point{930, 0}, Max: image.Point{1000, 100}}, gotErr: false},
		//宽高比小于70%的场景, 则是使用宽度计算高度
		{dx: 70, dy: 120, dxCenter: 70, dyCenter: 0, rect: image.Rectangle{Min: image.Point{0, 0}, Max: image.Point{70, 100}}, gotErr: false},
		{dx: 70, dy: 1000, dxCenter: 0, dyCenter: 0, rect: image.Rectangle{Min: image.Point{0, 0}, Max: image.Point{70, 100}}, gotErr: false},
		{dx: 70, dy: 1000, dxCenter: 0, dyCenter: 100, rect: image.Rectangle{Min: image.Point{0, 50}, Max: image.Point{70, 150}}, gotErr: false},
		//出错场景
		{dx: 0, dy: 123, gotErr: true},
		{dx: 123, dy: 0, gotErr: true},
	}

	for _, tst := range tests {
		rect, err := DetermineCutFrame(tst.dx, tst.dy, tst.dxCenter, tst.dyCenter, defaultAspectRatio)
		gotErr := err != nil
		assert.Equal(t, tst.gotErr, gotErr)
		assert.Equal(t, tst.rect, rect)
	}
}

func TestPigoRec(t *testing.T) {
	os.RemoveAll("./testdata/output_pigo/")
	os.MkdirAll("./testdata/output_pigo/", 0755)
	pg, err := pigo.NewPigo("../.vscode/tests/models")
	assert.NoError(t, err)
	face.SetFaceRec(pg)
	total := 0
	count := 0
	filepath.Walk("./testdata/input", func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if err != nil {
			return nil
		}
		raw, err := os.ReadFile(path)
		assert.NoError(t, err)
		total++
		out, err := CutImageWithFaceRecFromBytes(context.Background(), raw)
		if err != nil {
			return nil
		}
		count++
		assert.NoError(t, err)
		err = os.WriteFile("./testdata/output_pigo/"+filepath.Base(path), out, 0644)
		assert.NoError(t, err)
		return nil
	})
	t.Logf("total:%d, rec:%d", total, count)
	//total:17, rec:15
}
