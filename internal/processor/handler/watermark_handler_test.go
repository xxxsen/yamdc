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
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/store"
)

func TestWatermarkHandlerNilPoster(t *testing.T) {
	h := &watermark{storage: store.NewMemStorage()}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Poster: nil},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestWatermarkHandlerEmptyPosterKey(t *testing.T) {
	h := &watermark{storage: store.NewMemStorage()}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Poster: &model.File{Name: "poster.jpg"}},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestWatermarkHandlerNoTags(t *testing.T) {
	h := &watermark{storage: store.NewMemStorage()}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Poster: &model.File{Name: "poster.jpg", Key: "pkey"}},
	}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestWatermarkHandlerStorageError(t *testing.T) {
	storage := store.NewMemStorage()
	h := &watermark{storage: storage}
	num, _ := number.Parse("ABC-123-C")
	num.SetExternalFieldUncensor(true)
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Poster: &model.File{Name: "poster.jpg", Key: "nonexistent"}},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
}

func TestWatermarkHandlerWithValidImage(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()

	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	jpegData := buf.Bytes()
	require.NoError(t, storage.PutData(ctx, "posterkey", jpegData, 0))

	h := &watermark{storage: storage}
	num, _ := number.Parse("ABC-123-C")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Poster: &model.File{Name: "poster.jpg", Key: "posterkey"}},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotEmpty(t, fc.Meta.Poster.Key)
}

func TestWatermarkHandlerAllTagTypes(t *testing.T) {
	tests := []struct {
		name     string
		numberID string
		uncensor bool
	}{
		{"4k", "ABC-123-4K", false},
		{"8k", "ABC-123-8K", false},
		{"VR", "ABC-123-VR", false},
		{"chinese subtitle", "ABC-123-C", false},
		{"leak", "ABC-123-LEAK", false},
		{"hack", "ABC-123-UC", false},
		{"uncensor", "ABC-123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := store.NewMemStorage()
			h := &watermark{storage: storage}
			num, _ := number.Parse(tt.numberID)
			if tt.uncensor {
				num.SetExternalFieldUncensor(true)
			}
			fc := &model.FileContext{
				Number: num,
				Meta:   &model.MovieMeta{Poster: &model.File{Name: "poster.jpg", Key: "nonexistent"}},
			}
			err := h.Handle(context.Background(), fc)
			assert.Error(t, err)
		})
	}
}
