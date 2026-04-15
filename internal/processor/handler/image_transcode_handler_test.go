package handler

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"
)

func makeTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func TestImageTranscodeHandlerName(t *testing.T) {
	h := &imageTranscodeHandler{}
	assert.Equal(t, HImageTranscoder, h.Name())
}

func TestImageTranscodeHandlerNilFiles(t *testing.T) {
	h := &imageTranscodeHandler{storage: store.NewMemStorage()}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestImageTranscodeHandlerEmptyKeys(t *testing.T) {
	h := &imageTranscodeHandler{storage: store.NewMemStorage()}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Cover:  &model.File{Name: "cover.jpg"},
			Poster: &model.File{Name: "poster.jpg"},
			SampleImages: []*model.File{
				{Name: "sample.jpg"},
			},
		},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
	assert.Nil(t, fc.Meta.Cover)
	assert.Nil(t, fc.Meta.Poster)
	assert.Empty(t, fc.Meta.SampleImages)
}

func TestImageTranscodeHandlerStorageError(t *testing.T) {
	storage := store.NewMemStorage()
	h := &imageTranscodeHandler{storage: storage}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Cover: &model.File{Name: "cover.jpg", Key: "nonexistent_key"},
		},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
	assert.Nil(t, fc.Meta.Cover)
}

func TestImageTranscodeHandlerSuccess(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	jpegData := makeTestJPEG(t)
	require.NoError(t, storage.PutData(ctx, "coverkey", jpegData, 0))
	require.NoError(t, storage.PutData(ctx, "posterkey", jpegData, 0))
	require.NoError(t, storage.PutData(ctx, "samplekey", jpegData, 0))

	h := &imageTranscodeHandler{storage: storage}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
			Poster: &model.File{Name: "poster.jpg", Key: "posterkey"},
			SampleImages: []*model.File{
				{Name: "sample.jpg", Key: "samplekey"},
			},
		},
	}
	err := h.Handle(ctx, fc)
	assert.NoError(t, err)
	assert.NotNil(t, fc.Meta.Cover)
	assert.NotNil(t, fc.Meta.Poster)
	assert.Len(t, fc.Meta.SampleImages, 1)
}

func TestTranscodeNilFile(t *testing.T) {
	h := &imageTranscodeHandler{storage: store.NewMemStorage()}
	result := h.transcode(context.Background(), "test", nil)
	assert.Nil(t, result)
}

func TestTranscodeValidImage(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	jpegData := makeTestJPEG(t)
	require.NoError(t, storage.PutData(ctx, "imgkey", jpegData, 0))
	h := &imageTranscodeHandler{storage: storage}
	f := &model.File{Name: "img.jpg", Key: "imgkey"}
	result := h.transcode(ctx, "test", f)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Key)
}

func TestImageTranscodeHandlerWithSamples(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	jpegData := makeTestJPEG(t)
	require.NoError(t, storage.PutData(ctx, "s1", jpegData, 0))
	require.NoError(t, storage.PutData(ctx, "s2", jpegData, 0))

	h := &imageTranscodeHandler{storage: storage}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			SampleImages: []*model.File{
				{Name: "sample1.jpg", Key: "s1"},
				{Name: "sample2.jpg", Key: "s2"},
			},
		},
	}
	err := h.Handle(ctx, fc)
	assert.NoError(t, err)
	assert.Len(t, fc.Meta.SampleImages, 2)
}

func TestTranscodeInvalidImage(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	require.NoError(t, storage.PutData(ctx, "badkey", []byte("not an image"), 0))
	h := &imageTranscodeHandler{storage: storage}
	result := h.transcode(ctx, "test", &model.File{Name: "bad.jpg", Key: "badkey"})
	assert.Nil(t, result)
}
