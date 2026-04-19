package handler

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/store"
)

type mockFaceRec struct {
	faces     []image.Rectangle
	err       error
	callCount int
}

func (m *mockFaceRec) Name() string { return "mock_face" }
func (m *mockFaceRec) SearchFaces(_ context.Context, _ []byte) ([]image.Rectangle, error) {
	m.callCount++
	return m.faces, m.err
}

func makeTestImage(t *testing.T, w, h int) []byte { //nolint:unparam // 签名由接口 / 测试期望固定
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}))
	return buf.Bytes()
}

func TestPosterCropHandlerName(t *testing.T) {
	h := &posterCropHandler{}
	assert.Equal(t, HPosterCropper, h.Name())
}

func TestPosterCropHandlerSkipExistingPoster(t *testing.T) {
	h := &posterCropHandler{storage: store.NewMemStorage()}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Poster: &model.File{Name: "poster.jpg", Key: "pkey"},
			Cover:  &model.File{Name: "cover.jpg", Key: "ckey"},
		},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestPosterCropHandlerSkipNoCover(t *testing.T) {
	h := &posterCropHandler{storage: store.NewMemStorage()}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Number: "ABC-123"},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestPosterCropHandlerStorageError(t *testing.T) {
	storage := store.NewMemStorage()
	h := &posterCropHandler{storage: storage}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "nonexistent"},
		},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
}

func TestPosterCropHandlerCensorCutterNoFaceRec(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	h := &posterCropHandler{storage: storage}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}

func TestPosterCropHandlerCensorCutterWithFaceRecNoFaces(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	faceRec := &mockFaceRec{faces: nil}
	h := &posterCropHandler{storage: storage, faceRec: faceRec}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}

func TestPosterCropHandlerCensorCutterWithFaceRecOneFaceInRaw(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	faceRect := image.Rect(200, 100, 400, 400)
	faceRec := &sequentialFaceRec{
		results: []faceRecResult{
			{faces: nil},
			{faces: []image.Rectangle{faceRect}},
			{faces: []image.Rectangle{faceRect}},
		},
	}
	h := &posterCropHandler{storage: storage, faceRec: faceRec}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}

type faceRecResult struct {
	faces []image.Rectangle
	err   error
}

type sequentialFaceRec struct {
	results []faceRecResult
	idx     int
}

func (s *sequentialFaceRec) Name() string { return "sequential_face" }
func (s *sequentialFaceRec) SearchFaces(_ context.Context, _ []byte) ([]image.Rectangle, error) {
	if s.idx >= len(s.results) {
		return nil, nil
	}
	r := s.results[s.idx]
	s.idx++
	return r.faces, r.err
}

func TestPosterCropHandlerCensorCutterWithFaceRecError(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	faceRec := &mockFaceRec{err: errors.New("face rec error")}
	h := &posterCropHandler{storage: storage, faceRec: faceRec}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}

func TestPosterCropHandlerCensorCutterWithFaceRecFoundInCropped(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	faceRec := &mockFaceRec{
		faces: []image.Rectangle{image.Rect(10, 10, 100, 100)},
	}
	h := &posterCropHandler{storage: storage, faceRec: faceRec}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}

func TestPosterCropHandlerCensorCutterMultipleFacesInOriginal(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	faceRec := &sequentialFaceRec{
		results: []faceRecResult{
			{faces: nil},
			{faces: []image.Rectangle{image.Rect(10, 10, 50, 50), image.Rect(100, 100, 200, 200)}},
		},
	}
	h := &posterCropHandler{storage: storage, faceRec: faceRec}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}

func TestPosterCropHandlerUncensorCutterNoFaceRec(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	h := &posterCropHandler{storage: storage}
	num, _ := number.Parse("ABC-123")
	num.SetExternalFieldUncensor(true)
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}

func TestPosterCropHandlerUncensorCutterWithFaceRecError(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	faceRec := &mockFaceRec{err: errors.New("face rec error")}
	h := &posterCropHandler{storage: storage, faceRec: faceRec}
	num, _ := number.Parse("ABC-123")
	num.SetExternalFieldUncensor(true)
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}

func TestPosterCropHandlerCutterError(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	require.NoError(t, storage.PutData(ctx, "coverkey", []byte("not valid image data"), 0))

	h := &posterCropHandler{storage: storage}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	assert.Error(t, err)
}

func TestPosterCropHandlerCensorCutterWithFaceRecCutError(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	require.NoError(t, storage.PutData(ctx, "coverkey", []byte("bad data"), 0))

	faceRec := &mockFaceRec{}
	h := &posterCropHandler{storage: storage, faceRec: faceRec}
	cutter := h.censorCutter(ctx)
	_, err := cutter([]byte("bad data"))
	assert.Error(t, err)
}

func TestPosterCropHandlerUncensorCutterWithFaceRecSuccess(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	imgData := makeTestImage(t, 800, 600)
	require.NoError(t, storage.PutData(ctx, "coverkey", imgData, 0))

	faceRec := &mockFaceRec{
		faces: []image.Rectangle{image.Rect(200, 100, 400, 400)},
	}
	h := &posterCropHandler{storage: storage, faceRec: faceRec}
	num, _ := number.Parse("ABC-123")
	num.SetExternalFieldUncensor(true)
	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
		},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotNil(t, fc.Meta.Poster)
}
