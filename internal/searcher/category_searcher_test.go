package searcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
)

func TestCategorySearcher(t *testing.T) {
	cs := NewCategorySearcher(nil, nil)
	assert.Equal(t, "category", cs.Name())
	require.ErrorIs(t, cs.Check(context.Background()), errCheckOnCategorySearcher)
}

func TestCategorySearcher_UseDefaultChain(t *testing.T) {
	s1 := &mockSearcher{
		name: "s1",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return &model.MovieMeta{Title: "default"}, true, nil
		},
	}
	cs := NewCategorySearcher([]ISearcher{s1}, nil)
	num := mustParseNumber(t, "ABC-123")
	meta, found, err := cs.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "default", meta.Title)
}

func TestCategorySearcher_UseCategoryChain(t *testing.T) {
	s1 := &mockSearcher{
		name: "default",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return &model.MovieMeta{Title: "default"}, true, nil
		},
	}
	s2 := &mockSearcher{
		name: "cat-searcher",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return &model.MovieMeta{Title: "cat-result"}, true, nil
		},
	}
	cs := NewCategorySearcher([]ISearcher{s1}, map[string][]ISearcher{
		"MYCAT": {s2},
	})
	num := mustParseNumber(t, "ABC-123")
	num.SetExternalFieldCategory("mycat")
	meta, found, err := cs.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "cat-result", meta.Title)
}

func TestCategorySearcher_FallbackToDefaultWhenCatNotFound(t *testing.T) {
	s1 := &mockSearcher{
		name: "default",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return &model.MovieMeta{Title: "default"}, true, nil
		},
	}
	cs := NewCategorySearcher([]ISearcher{s1}, nil)
	num := mustParseNumber(t, "ABC-123")
	num.SetExternalFieldCategory("UNKNOWN")
	meta, found, err := cs.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "default", meta.Title)
}

func TestCategorySearcher_Swap(t *testing.T) {
	s1 := &mockSearcher{
		name: "old",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return &model.MovieMeta{Title: "old"}, true, nil
		},
	}
	s2 := &mockSearcher{
		name: "new",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return &model.MovieMeta{Title: "new"}, true, nil
		},
	}
	cs := NewCategorySearcher([]ISearcher{s1}, nil)
	cs.Swap([]ISearcher{s2}, nil)
	num := mustParseNumber(t, "ABC-123")
	meta, found, err := cs.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "new", meta.Title)
}
