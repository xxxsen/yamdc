package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
)

type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

func TestEncodeNumberID(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal", "ABC-123", "ABC%2D123"},
		{"with underscore", "ABC_123", "ABC%5F123"},
		{"no special", "ABC123", "ABC123"},
		{"mixed", "A-B_C", "A%2DB%5FC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, c.encodeNumberID(tt.input))
		})
	}
}

func TestCleanSearchTitle(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with brackets prefix",
			input:    "[Something] ABC-123 Nice Title (中文字幕)",
			expected: "ABC-123 Nice Title",
		},
		{
			name:     "without brackets",
			input:    "ABC-123 Nice Title",
			expected: "ABC-123 Nice Title",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "no match",
			input:    "random text",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, c.cleanSearchTitle(tt.input))
		})
	}
}

func TestReadTitleFromCNumber(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	c.m["TEST-999"] = "Test Chinese Title"

	t.Run("found", func(t *testing.T) {
		title, ok, err := c.readTitleFromCNumber(context.Background(), "TEST-999")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "Test Chinese Title", title)
	})

	t.Run("not found", func(t *testing.T) {
		title, ok, err := c.readTitleFromCNumber(context.Background(), "NONEXIST-123")
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Empty(t, title)
	})
}

func TestReadTitleFromCNumberUninitialized(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{}
	c.tryInitCNumber(context.Background())
	_, ok, err := c.readTitleFromCNumber(context.Background(), "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestReadTitleFromYesJavHTTPError(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{err: errors.New("network error")},
	}
	_, ok, err := c.readTitleFromYesJav(context.Background(), "ABC-123")
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestReadTitleFromYesJavNoMatch(t *testing.T) {
	html := `<html><body><font size="+0.5"><a target="_blank">UNRELATED CONTENT</a></font></body></html>`
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
			},
		},
	}
	title, ok, err := c.readTitleFromYesJav(context.Background(), "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, title)
}

func TestReadTitleFromYesJavMatch(t *testing.T) {
	html := `<html><body><font size="+0.5"><a target="_blank">ABC-123 Nice Title (中文字幕)</a></font></body></html>`
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
			},
		},
	}
	title, ok, err := c.readTitleFromYesJav(context.Background(), "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "ABC-123 NICE TITLE", title)
}

func TestReadTitleFromYesJavMismatchNumberID(t *testing.T) {
	html := `<html><body><font size="+0.5"><a target="_blank">XYZ-999 Wrong Number (中文字幕)</a></font></body></html>`
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
			},
		},
	}
	title, ok, err := c.readTitleFromYesJav(context.Background(), "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, title)
}

func TestReadTitleFromYesJavNoChinese(t *testing.T) {
	html := `<html><body><font size="+0.5"><a target="_blank">ABC-123 Title Without Chinese Marker</a></font></body></html>`
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
			},
		},
	}
	title, ok, err := c.readTitleFromYesJav(context.Background(), "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, title)
}

func TestReadTitleFromYesJavBodyReadError(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(&errReader{}),
			},
		},
	}
	_, ok, err := c.readTitleFromYesJav(context.Background(), "ABC-123")
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestReadTitleFromYesJavEmptyContent(t *testing.T) {
	html := `<html><body><font size="+0.5"><a target="_blank">   </a></font></body></html>`
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
			},
		},
	}
	title, ok, err := c.readTitleFromYesJav(context.Background(), "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, title)
}

func TestReadTitleFromYesJavMultipleLinks(t *testing.T) {
	html := `<html><body><font size="+0.5">
		<a target="_blank">XYZ-999 Wrong (中文字幕)</a>
		<a target="_blank">ABC-123 Correct Title (中文字幕)</a>
	</font></body></html>`
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
			},
		},
	}
	title, ok, err := c.readTitleFromYesJav(context.Background(), "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, title, "ABC-123")
}

func TestChineseTitleOptimizeHandleWithCNumber(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{err: errors.New("should not reach")},
	}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	c.m["ABC-123"] = "Chinese Title"

	num, _ := number.Parse("ABC-123")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Number: "ABC-123"},
	}
	err := c.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Equal(t, "Chinese Title", fc.Meta.TitleTranslated)
}

func TestChineseTitleOptimizeHandleNoResult(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
			},
		},
	}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	num, _ := number.Parse("XYZ-999")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Number: "XYZ-999"},
	}
	err := c.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Empty(t, fc.Meta.TitleTranslated)
}

func TestChineseTitleOptimizeHandleFallbackToYesJav(t *testing.T) {
	html := `<html><body><font size="+0.5"><a target="_blank">DEF-456 Found Title (中文字幕)</a></font></body></html>`
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
			},
		},
	}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	num, _ := number.Parse("DEF-456")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Number: "DEF-456"},
	}
	err := c.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Equal(t, "DEF-456 FOUND TITLE", fc.Meta.TitleTranslated)
}

func TestChineseTitleOptimizeHandleSubHandlerError(t *testing.T) {
	c := &chineseTitleTranslateOptimizer{
		cli: &mockHTTPClient{
			err: errors.New("network error"),
		},
	}
	c.tryInitCNumber(context.Background())
	if c.m == nil {
		c.m = make(map[string]string)
	}
	num, _ := number.Parse("GHI-789")
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{Number: "GHI-789"},
	}
	err := c.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Empty(t, fc.Meta.TitleTranslated)
}
