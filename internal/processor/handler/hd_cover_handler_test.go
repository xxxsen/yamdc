package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/store"
)

// TestHDCoverLinkTemplateShape 验证运行时解码得到的模板具备我们依赖的
// 结构特征 — 以 `https://` 开头、含两个 `%s` 占位符、可成功拼接成合法 URL。
// 不断言具体 host, 保证模板替换不会破这个 handler 的前提假设。
func TestHDCoverLinkTemplateShape(t *testing.T) {
	require.True(t, strings.HasPrefix(defaultHDCoverLinkTemplate, "https://"),
		"template must start with https://, got %q", defaultHDCoverLinkTemplate)
	require.Equal(t, 2, strings.Count(defaultHDCoverLinkTemplate, "%s"),
		"template must contain exactly two %%s placeholders, got %q", defaultHDCoverLinkTemplate)
	url := fmt.Sprintf(defaultHDCoverLinkTemplate, "abc00123", "abc00123")
	require.True(t, strings.HasPrefix(url, "https://"), "resolved url must be https")
	require.NotContains(t, url, "%s", "resolved url must have no remaining placeholders")
}

type mockHTTPClient struct {
	resp *http.Response
	err  error
}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestHDCoverHandlerSkipNonTwoPart(t *testing.T) {
	tests := []struct {
		name     string
		numberID string
	}{
		{"single part", "ABC123"},
		{"three parts", "ABC-DEF-123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &highQualityCoverHandler{}
			num, _ := number.Parse(tt.numberID)
			fc := &model.FileContext{
				Number: num,
				Meta:   &model.MovieMeta{Cover: &model.File{Name: "c.jpg", Key: "ck"}},
			}
			err := h.Handle(context.Background(), fc)
			assert.NoError(t, err)
		})
	}
}

func TestHDCoverHandlerHTTPError(t *testing.T) {
	h := &highQualityCoverHandler{
		httpClient: &mockHTTPClient{err: assert.AnError},
		storage:    store.NewMemStorage(),
	}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Cover: &model.File{Name: "c.jpg", Key: "ck"}},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
}

func TestHDCoverHandlerNon200(t *testing.T) {
	h := &highQualityCoverHandler{
		httpClient: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("")),
			},
		},
		storage: store.NewMemStorage(),
	}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Cover: &model.File{Name: "c.jpg", Key: "ck"}},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errHDCoverResponseNotOK)
}

func TestHDCoverHandlerTooSmall(t *testing.T) {
	h := &highQualityCoverHandler{
		httpClient: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("tiny")),
			},
		},
		storage: store.NewMemStorage(),
	}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Cover: &model.File{Name: "c.jpg", Key: "ck"}},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errHDCoverTooSmall)
}

func TestHDCoverHandlerNonImageData(t *testing.T) {
	bigData := strings.Repeat("x", defaultMinCoverSize+1)
	h := &highQualityCoverHandler{
		httpClient: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(bigData)),
			},
		},
		storage: store.NewMemStorage(),
	}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Cover: &model.File{Name: "c.jpg", Key: "ck"}},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
}

func TestHDCoverHandlerBodyReadError(t *testing.T) {
	h := &highQualityCoverHandler{
		httpClient: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(&errBodyReader{}),
			},
		},
		storage: store.NewMemStorage(),
	}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Cover: &model.File{Name: "c.jpg", Key: "ck"}},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
}

type errBodyReader struct{}

func (e *errBodyReader) Read(_ []byte) (int, error) {
	return 0, errors.New("body read error")
}

func TestHDCoverHandlerValidImage(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()

	img := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
	for x := 0; x < 1000; x++ {
		for y := 0; y < 1000; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: uint8((x + y) % 256), A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}))
	jpegData := buf.Bytes()

	if len(jpegData) < defaultMinCoverSize {
		t.Skipf("test image too small: %d < %d", len(jpegData), defaultMinCoverSize)
	}

	h := &highQualityCoverHandler{
		httpClient: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(jpegData)),
			},
		},
		storage: storage,
	}
	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Cover: &model.File{Name: "c.jpg", Key: "ck"}},
	}
	err := h.Handle(ctx, fc)
	require.NoError(t, err)
	assert.NotEqual(t, "ck", fc.Meta.Cover.Key)
}
