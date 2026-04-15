package image

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noSubImage is an image.Image without SubImage (unlike *image.RGBA).
type noSubImage struct{}

func (noSubImage) Bounds() image.Rectangle { return image.Rect(0, 0, 20, 20) }
func (noSubImage) ColorModel() color.Model  { return color.RGBAModel }
func (noSubImage) At(x, y int) color.Color   { return color.RGBA{A: 255} }

func TestDetermineCutFrame_invalidResolution(t *testing.T) {
	t.Parallel()
	_, err := DetermineCutFrame(0, 10, 5, 5, 1)
	assert.ErrorIs(t, err, errInvalidResolution)
	_, err = DetermineCutFrame(10, 0, 5, 5, 1)
	assert.ErrorIs(t, err, errInvalidResolution)
}

func TestDetermineCutFrame_invalidAspectRatio(t *testing.T) {
	t.Parallel()
	_, err := DetermineCutFrame(100, 100, 50, 50, 0)
	assert.ErrorIs(t, err, errInvalidAspectRatio)
}

func TestDetermineCutFrame_viaHeight_success(t *testing.T) {
	t.Parallel()
	// dx/dy > aspectRatio triggers height-based crop
	r, err := DetermineCutFrame(200, 100, 100, 50, 0.5)
	require.NoError(t, err)
	assert.Equal(t, 0, r.Min.Y)
	assert.Equal(t, 100, r.Max.Y)
	assert.Greater(t, r.Dx(), 0)
	assert.LessOrEqual(t, r.Max.X, 200)
}

func TestDetermineCutFrame_viaHeight_shiftLeft(t *testing.T) {
	t.Parallel()
	r, err := DetermineCutFrame(100, 50, 10, 25, 0.3)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, r.Min.X, 0)
	assert.LessOrEqual(t, r.Max.X, 100)
}

func TestDetermineCutFrame_viaWidth_success(t *testing.T) {
	t.Parallel()
	// dx/dy <= aspectRatio uses width path
	r, err := DetermineCutFrame(50, 200, 25, 100, 2.0)
	require.NoError(t, err)
	assert.Equal(t, 0, r.Min.X)
	assert.Equal(t, 50, r.Max.X)
	assert.Greater(t, r.Dy(), 0)
}

func TestDetermineCutFrameViaHeight_unsatisfied(t *testing.T) {
	t.Parallel()
	// After shifting to fit dx, crop window still invalid (left becomes negative).
	_, err := determineCutFrameViaHeight(30, 100, 15, 0.5)
	assert.ErrorIs(t, err, errCropUnsatisfied)
}

func TestDetermineCutFrameViaWidth_unsatisfied(t *testing.T) {
	t.Parallel()
	_, err := determineCutFrameViaWidth(25, 100, 10, 0.1)
	assert.ErrorIs(t, err, errCropUnsatisfied)
}

func TestCutImageViaRectangle_invalidRect(t *testing.T) {
	t.Parallel()
	img := MakeColorImage(image.Rect(0, 0, 10, 10), color.RGBA{A: 255})
	_, err := CutImageViaRectangle(img, image.Rect(0, 0, 20, 20))
	assert.ErrorIs(t, err, errInvalidRectangle)
}

func TestCutImageViaRectangle_noSubImage(t *testing.T) {
	t.Parallel()
	_, err := CutImageViaRectangle(noSubImage{}, image.Rect(0, 0, 10, 10))
	assert.ErrorIs(t, err, errNoSubImageSupport)
}

func TestCutImageViaRectangle_success(t *testing.T) {
	t.Parallel()
	img := MakeColorImage(image.Rect(0, 0, 40, 30), color.RGBA{R: 255, A: 255})
	out, err := CutImageViaRectangle(img, image.Rect(5, 5, 25, 20))
	require.NoError(t, err)
	assert.Equal(t, 20, out.Bounds().Dx())
	assert.Equal(t, 15, out.Bounds().Dy())
}

func TestCutCensoredImage(t *testing.T) {
	t.Parallel()
	img := MakeColorImage(image.Rect(0, 0, 600, 900), color.RGBA{G: 128, A: 255})
	out, err := CutCensoredImage(img)
	require.NoError(t, err)
	ar := float64(out.Bounds().Dx()) / float64(out.Bounds().Dy())
	assert.InDelta(t, defaultAspectRatio, ar, 0.02)
}

func TestCutCensoredImage_determineFrameError(t *testing.T) {
	t.Parallel()
	// Zero width triggers invalid resolution before crop.
	img := image.NewRGBA(image.Rect(0, 0, 0, 40))
	_, err := CutCensoredImage(img)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errInvalidResolution)
}

func TestCutCensoredImageFromBytes(t *testing.T) {
	t.Parallel()
	data, err := MakeColorImageData(image.Rect(0, 0, 400, 600), color.RGBA{B: 200, A: 255})
	require.NoError(t, err)
	out, err := CutCensoredImageFromBytes(data)
	require.NoError(t, err)
	assert.NotEmpty(t, out)

	_, err = CutCensoredImageFromBytes([]byte("bad"))
	assert.Error(t, err)
}

type mockFaceRec struct {
	rects []image.Rectangle
	err   error
}

func (m *mockFaceRec) Name() string { return "mock" }

func (m *mockFaceRec) SearchFaces(ctx context.Context, data []byte) ([]image.Rectangle, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.rects, nil
}

func TestCutImageWithFaceRecUsing_nilRecognizer(t *testing.T) {
	t.Parallel()
	img := MakeColorImage(image.Rect(0, 0, 100, 100), color.RGBA{A: 255})
	_, err := CutImageWithFaceRecUsing(context.Background(), nil, img)
	assert.ErrorIs(t, err, errNoFaceRecognizer)
}

func TestCutImageWithFaceRecUsing_searchError(t *testing.T) {
	t.Parallel()
	img := MakeColorImage(image.Rect(0, 0, 200, 300), color.RGBA{A: 255})
	rec := &mockFaceRec{err: errors.New("vision down")}
	_, err := CutImageWithFaceRecUsing(context.Background(), rec, img)
	assert.Error(t, err)
}

func TestCutImageWithFaceRecUsing_noFaces(t *testing.T) {
	t.Parallel()
	img := MakeColorImage(image.Rect(0, 0, 200, 300), color.RGBA{A: 255})
	rec := &mockFaceRec{rects: nil}
	_, err := CutImageWithFaceRecUsing(context.Background(), rec, img)
	assert.ErrorIs(t, err, errNoFaceFound)
}

func TestCutImageWithFaceRecUsing_success(t *testing.T) {
	t.Parallel()
	img := MakeColorImage(image.Rect(0, 0, 800, 1200), color.RGBA{R: 40, A: 255})
	faceBox := image.Rect(300, 400, 500, 700) // center ~400,550
	rec := &mockFaceRec{rects: []image.Rectangle{faceBox}}
	out, err := CutImageWithFaceRecUsing(context.Background(), rec, img)
	require.NoError(t, err)
	assert.Positive(t, out.Bounds().Dx())
	assert.Positive(t, out.Bounds().Dy())
}

func TestCutImageWithFaceRecFromBytesWithFaceRec(t *testing.T) {
	t.Parallel()
	data, err := MakeColorImageData(image.Rect(0, 0, 400, 600), color.RGBA{A: 255})
	require.NoError(t, err)
	faceBox := image.Rect(150, 200, 250, 350)
	rec := &mockFaceRec{rects: []image.Rectangle{faceBox}}

	out, err := CutImageWithFaceRecFromBytesWithFaceRec(context.Background(), rec, data)
	require.NoError(t, err)
	assert.NotEmpty(t, out)

	_, err = CutImageWithFaceRecFromBytesWithFaceRec(context.Background(), rec, []byte("x"))
	assert.Error(t, err)

	badRec := &mockFaceRec{rects: nil}
	_, err = CutImageWithFaceRecFromBytesWithFaceRec(context.Background(), badRec, data)
	assert.Error(t, err)
}
