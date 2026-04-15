package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"
)

type mockTranslator struct {
	result string
	err    error
}

func (m *mockTranslator) Name() string { return "mock" }
func (m *mockTranslator) Translate(_ context.Context, _, _, _ string) (string, error) {
	return m.result, m.err
}

func TestTranslateHandlerNilDeps(t *testing.T) {
	h := &translaterHandler{}
	fc := &model.FileContext{Meta: &model.MovieMeta{Title: "test"}}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestTranslateHandlerNilTranslator(t *testing.T) {
	h := &translaterHandler{storage: store.NewMemStorage()}
	fc := &model.FileContext{Meta: &model.MovieMeta{Title: "test"}}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestTranslateHandlerNilStorage(t *testing.T) {
	h := &translaterHandler{translator: &mockTranslator{result: "translated"}}
	fc := &model.FileContext{Meta: &model.MovieMeta{Title: "test"}}
	err := h.Handle(context.Background(), fc)
	assert.NoError(t, err)
}

func TestIsNeedTranslate(t *testing.T) {
	h := &translaterHandler{}
	tests := []struct {
		name string
		lang string
		want bool
	}{
		{"empty lang", "", false},
		{"zh-cn", enum.MetaLangZH, false},
		{"zh-tw", enum.MetaLangZHTW, false},
		{"ja", enum.MetaLangJa, true},
		{"en", enum.MetaLangEn, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, h.isNeedTranslate(tt.lang))
		})
	}
}

func TestTranslateSingle(t *testing.T) {
	storage := store.NewMemStorage()

	tests := []struct {
		name       string
		translator *mockTranslator
		input      string
		lang       string
		wantResult string
		wantErr    bool
	}{
		{
			name:       "empty input skipped",
			translator: &mockTranslator{},
			input:      "",
			lang:       "ja",
			wantResult: "",
		},
		{
			name:       "chinese not translated",
			translator: &mockTranslator{},
			input:      "some text",
			lang:       enum.MetaLangZH,
			wantResult: "",
		},
		{
			name:       "successful translation",
			translator: &mockTranslator{result: "翻译结果"},
			input:      "test input",
			lang:       "ja",
			wantResult: "翻译结果",
		},
		{
			name:       "translation error",
			translator: &mockTranslator{err: errors.New("translate failed")},
			input:      "test",
			lang:       "en",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &translaterHandler{
				storage:    storage,
				translator: tt.translator,
			}
			var out string
			err := h.translateSingle(context.Background(), "test", tt.input, tt.lang, &out)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantResult, out)
			}
		})
	}
}

func TestTranslateSingleUsesCache(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	h := &translaterHandler{
		storage:    storage,
		translator: &mockTranslator{result: "from translator"},
	}

	var out string
	err := h.translateSingle(ctx, "test", "input text", "ja", &out)
	require.NoError(t, err)
	assert.Equal(t, "from translator", out)

	h.translator = &mockTranslator{err: errors.New("should not be called")}
	var out2 string
	err = h.translateSingle(ctx, "test", "input text", "ja", &out2)
	require.NoError(t, err)
	assert.Equal(t, "from translator", out2)
}

func TestTranslateArray(t *testing.T) {
	storage := store.NewMemStorage()

	t.Run("skips when not needed", func(t *testing.T) {
		h := &translaterHandler{storage: storage, translator: &mockTranslator{}}
		var out []string
		h.translateArray(context.Background(), "test", []string{"a"}, enum.MetaLangZH, &out)
		assert.Nil(t, out)
	})

	t.Run("translates and appends", func(t *testing.T) {
		h := &translaterHandler{
			storage:    storage,
			translator: &mockTranslator{result: "translated"},
		}
		var out []string
		h.translateArray(context.Background(), "test", []string{"a", "b"}, "ja", &out)
		assert.Contains(t, out, "a")
		assert.Contains(t, out, "b")
		assert.Contains(t, out, "translated")
	})

	t.Run("error in translate does not crash", func(t *testing.T) {
		freshStorage := store.NewMemStorage()
		h := &translaterHandler{
			storage:    freshStorage,
			translator: &mockTranslator{err: errors.New("fail")},
		}
		var out []string
		h.translateArray(context.Background(), "test", []string{"unique_untranslated"}, "ja", &out)
		assert.Contains(t, out, "unique_untranslated")
		assert.Len(t, out, 1)
	})
}

func TestTranslateHandlerHandle(t *testing.T) {
	storage := store.NewMemStorage()
	translator := &mockTranslator{result: "translated"}

	h := &translaterHandler{storage: storage, translator: translator}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title:      "original title",
			TitleLang:  "ja",
			Plot:       "original plot",
			PlotLang:   "ja",
			Genres:     []string{"genre1"},
			GenresLang: "ja",
			Actors:     []string{"actor1"},
			ActorsLang: "ja",
		},
	}
	err := h.Handle(context.Background(), fc)
	require.NoError(t, err)
	assert.Equal(t, "translated", fc.Meta.TitleTranslated)
	assert.Equal(t, "translated", fc.Meta.PlotTranslated)
}

func TestTranslateHandlerHandleTitleError(t *testing.T) {
	storage := store.NewMemStorage()
	translator := &mockTranslator{err: errors.New("fail")}
	h := &translaterHandler{storage: storage, translator: translator}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title:     "test",
			TitleLang: "ja",
		},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
}

func TestBuildKey(t *testing.T) {
	h := &translaterHandler{}
	k1 := h.buildKey("hello")
	k2 := h.buildKey("hello")
	k3 := h.buildKey("world")
	assert.Equal(t, k1, k2)
	assert.NotEqual(t, k1, k3)
	assert.Contains(t, k1, "yamdc:translate:")
}

func TestTranslateArrayEmpty(t *testing.T) {
	storage := store.NewMemStorage()
	h := &translaterHandler{storage: storage, translator: &mockTranslator{result: "translated"}}
	var out []string
	h.translateArray(context.Background(), "test", nil, "ja", &out)
	assert.Empty(t, out)
}

func TestTranslateArrayMultipleItems(t *testing.T) {
	storage := store.NewMemStorage()
	h := &translaterHandler{storage: storage, translator: &mockTranslator{result: "翻译"}}
	var out []string
	h.translateArray(context.Background(), "genres", []string{"action", "drama"}, "ja", &out)
	assert.Contains(t, out, "action")
	assert.Contains(t, out, "drama")
	assert.Contains(t, out, "翻译")
	assert.Len(t, out, 4)
}

func TestTranslateHandleWithPlotError(t *testing.T) {
	callCount := 0
	translator := &countingTranslator{
		results: []translateResult{
			{result: "translated title", err: nil},
			{result: "", err: errors.New("plot translation failed")},
		},
		callCount: &callCount,
	}
	h := &translaterHandler{
		storage:    store.NewMemStorage(),
		translator: translator,
	}
	fc := &model.FileContext{
		Meta: &model.MovieMeta{
			Title:     "test title",
			TitleLang: "ja",
			Plot:      "test plot",
			PlotLang:  "ja",
		},
	}
	err := h.Handle(context.Background(), fc)
	assert.Error(t, err)
}

type translateResult struct {
	result string
	err    error
}

type countingTranslator struct {
	results   []translateResult
	callCount *int
}

func (c *countingTranslator) Name() string { return "counting" }
func (c *countingTranslator) Translate(_ context.Context, _, _, _ string) (string, error) {
	idx := *c.callCount
	*c.callCount++
	if idx < len(c.results) {
		return c.results[idx].result, c.results[idx].err
	}
	return "", errors.New("unexpected call")
}

type putFailStorage struct {
	store.IStorage
	inner store.IStorage
}

func (s *putFailStorage) GetData(ctx context.Context, key string) ([]byte, error) {
	return s.inner.GetData(ctx, key)
}

func (s *putFailStorage) PutData(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return errors.New("storage write error")
}

func (s *putFailStorage) IsDataExist(ctx context.Context, key string) (bool, error) {
	return s.inner.IsDataExist(ctx, key)
}

func TestTranslateSingleCacheWriteError(t *testing.T) {
	h := &translaterHandler{
		storage:    &putFailStorage{inner: store.NewMemStorage()},
		translator: &mockTranslator{result: "translated"},
	}
	var out string
	err := h.translateSingle(context.Background(), "test", "input", "ja", &out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache translate result failed")
}

func TestTranslateHandlerName(t *testing.T) {
	h := &translaterHandler{}
	assert.Equal(t, HTranslater, h.Name())
}
