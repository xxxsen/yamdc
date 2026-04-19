package handler

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yimage "github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/tag"
)

// newWatermarkHandler 构造一个默认规则表的 watermark handler, 供测试使用.
func newWatermarkHandler(storage store.IStorage) *watermark {
	return &watermark{storage: storage, rules: defaultWatermarkRules}
}

func TestWatermarkHandlerNilPoster(t *testing.T) {
	h := newWatermarkHandler(store.NewMemStorage())
	fc := &model.FileContext{Meta: &model.MovieMeta{Poster: nil}}
	assert.NoError(t, h.Handle(context.Background(), fc))
}

func TestWatermarkHandlerNilMeta(t *testing.T) {
	h := newWatermarkHandler(store.NewMemStorage())
	fc := &model.FileContext{Meta: nil}
	assert.NoError(t, h.Handle(context.Background(), fc))
}

func TestWatermarkHandlerEmptyPosterKey(t *testing.T) {
	h := newWatermarkHandler(store.NewMemStorage())
	fc := &model.FileContext{
		Meta: &model.MovieMeta{Poster: &model.File{Name: "poster.jpg"}},
	}
	assert.NoError(t, h.Handle(context.Background(), fc))
}

func TestWatermarkHandlerNoTags(t *testing.T) {
	h := newWatermarkHandler(store.NewMemStorage())
	fc := &model.FileContext{
		Meta: &model.MovieMeta{Poster: &model.File{Name: "poster.jpg", Key: "pkey"}},
	}
	assert.NoError(t, h.Handle(context.Background(), fc))
}

func TestWatermarkHandlerStorageError(t *testing.T) {
	h := newWatermarkHandler(store.NewMemStorage())
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Poster: &model.File{Name: "poster.jpg", Key: "nonexistent"},
			Genres: []string{tag.Unrated},
		},
	}
	assert.Error(t, h.Handle(context.Background(), fc))
}

func TestWatermarkHandlerWithValidImage(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()

	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	require.NoError(t, storage.PutData(ctx, "posterkey", buf.Bytes(), 0))

	h := newWatermarkHandler(storage)
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Poster: &model.File{Name: "poster.jpg", Key: "posterkey"},
			Genres: []string{tag.ChineseSubtitle},
		},
	}
	require.NoError(t, h.Handle(ctx, fc))
	assert.NotEmpty(t, fc.Meta.Poster.Key)
}

func TestWatermarkHandlerAllTagTypes(t *testing.T) {
	tests := []struct {
		name   string
		genres []string
	}{
		{"4k", []string{tag.Res4K}},
		{"8k", []string{tag.Res8K}},
		{"VR", []string{tag.VR}},
		{"chinese subtitle", []string{tag.ChineseSubtitle}},
		{"special_edition", []string{tag.SpecialEdition}},
		{"restored", []string{tag.Restored}},
		{"unrated", []string{tag.Unrated}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newWatermarkHandler(store.NewMemStorage())
			fc := &model.FileContext{
				Meta: &model.MovieMeta{
					Poster: &model.File{Name: "poster.jpg", Key: "nonexistent"},
					Genres: tt.genres,
				},
			}
			// 存储不存在对应 key, 预期在写回阶段报错,
			// 但此路径已经证明 matchWatermarks 正确命中了水印.
			assert.Error(t, h.Handle(context.Background(), fc))
		})
	}
}

// --- matchWatermarks 单元测试 (绕开 storage / image) ---

func TestMatchWatermarksOrder(t *testing.T) {
	h := newWatermarkHandler(nil)
	// 所有 tag 都命中时, 顺序严格等于 defaultWatermarkRules.
	genres := []string{
		tag.Restored, tag.SpecialEdition, tag.ChineseSubtitle,
		tag.Unrated, tag.VR, tag.Res8K, tag.Res4K,
	}
	got := h.matchWatermarks(genres)
	expect := []yimage.Watermark{
		yimage.WM4K, yimage.WM8K, yimage.WMVR,
		yimage.WMUnrated, yimage.WMChineseSubtitle,
		yimage.WMSpecialEdition, yimage.WMRestored,
	}
	assert.Equal(t, expect, got)
}

func TestMatchWatermarksPartial(t *testing.T) {
	h := newWatermarkHandler(nil)
	got := h.matchWatermarks([]string{tag.Res4K, tag.ChineseSubtitle})
	assert.Equal(t, []yimage.Watermark{yimage.WM4K, yimage.WMChineseSubtitle}, got)
}

func TestMatchWatermarksCaseInsensitive(t *testing.T) {
	h := newWatermarkHandler(nil)
	got := h.matchWatermarks([]string{"4k", "vr"})
	assert.Equal(t, []yimage.Watermark{yimage.WM4K, yimage.WMVR}, got)
}

func TestMatchWatermarksEmpty(t *testing.T) {
	h := newWatermarkHandler(nil)
	assert.Nil(t, h.matchWatermarks(nil))
	assert.Nil(t, h.matchWatermarks([]string{}))
}

func TestMatchWatermarksUnknownTag(t *testing.T) {
	h := newWatermarkHandler(nil)
	assert.Empty(t, h.matchWatermarks([]string{"Cosplay", "JK", "AI-泳装"}))
}

func TestMatchWatermarksDedup(t *testing.T) {
	h := newWatermarkHandler(nil)
	// Genres 里同一 tag 重复出现, 水印只出现一次.
	got := h.matchWatermarks([]string{tag.Res4K, tag.Res4K, tag.Res4K})
	assert.Equal(t, []yimage.Watermark{yimage.WM4K}, got)
}

func TestMatchWatermarksMixedCaseDedup(t *testing.T) {
	h := newWatermarkHandler(nil)
	// "4K" 和 "4k" 大小写不同, 都命中 tag.Res4K, 水印只画一次.
	got := h.matchWatermarks([]string{"4K", "4k"})
	assert.Equal(t, []yimage.Watermark{yimage.WM4K}, got)
}

func TestMatchWatermarksSharedWatermark(t *testing.T) {
	// 注入自定义 rules: 两个不同 tag 指向同一 watermark.
	// 模拟"4K / 8K 都打 HD 水印"的场景, 这里借用 yimage.WM4K 当
	// HD slot, 只是为了验证 watermark 级去重.
	h := &watermark{
		rules: []watermarkRule{
			{tag.Res4K, yimage.WM4K},
			{tag.Res8K, yimage.WM4K},
			{tag.VR, yimage.WMVR},
		},
	}
	got := h.matchWatermarks([]string{tag.Res4K, tag.Res8K, tag.VR})
	// WM4K 只出现一次, 位置 = 第一条命中 rule (tag.Res4K) 的位置.
	assert.Equal(t, []yimage.Watermark{yimage.WM4K, yimage.WMVR}, got)
}

func TestMatchWatermarksSharedWatermarkSingleHit(t *testing.T) {
	h := &watermark{
		rules: []watermarkRule{
			{tag.Res4K, yimage.WM4K},
			{tag.Res8K, yimage.WM4K},
		},
	}
	// 只命中 Res8K, 水印正常出现一次 (去重不会吞掉合法命中).
	got := h.matchWatermarks([]string{tag.Res8K})
	assert.Equal(t, []yimage.Watermark{yimage.WM4K}, got)
}

func TestMatchWatermarksCustomRules(t *testing.T) {
	h := &watermark{
		rules: []watermarkRule{
			{tag.VR, yimage.WMVR},
			{tag.Restored, yimage.WMRestored},
		},
	}
	// 命中两条, 其它 tag 被忽略.
	got := h.matchWatermarks([]string{tag.VR, tag.Restored, tag.Res4K})
	assert.Equal(t, []yimage.Watermark{yimage.WMVR, yimage.WMRestored}, got)
}
