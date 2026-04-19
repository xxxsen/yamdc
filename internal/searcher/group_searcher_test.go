package searcher

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
)

func TestGroupSearcher(t *testing.T) {
	g := NewGroup(nil)
	assert.Equal(t, "group", g.Name())
	require.ErrorIs(t, g.Check(context.Background()), errCheckOnGroupSearcher)
}

func TestGroupSearch_NoSearchers(t *testing.T) {
	g := NewGroup(nil)
	num := mustParseNumber(t, "ABC-123")
	_, found, err := g.Search(context.Background(), num)
	require.NoError(t, err)
	require.False(t, found)
}

func TestGroupSearch_FirstFound(t *testing.T) {
	s1 := &mockSearcher{
		name: "s1",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return &model.MovieMeta{Title: "found"}, true, nil
		},
	}
	g := NewGroup([]ISearcher{s1})
	num := mustParseNumber(t, "ABC-123")
	meta, found, err := g.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "found", meta.Title)
}

func TestGroupSearch_SkipsError(t *testing.T) {
	s1 := &mockSearcher{
		name: "s1",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return nil, false, errors.New("s1 error")
		},
	}
	s2 := &mockSearcher{
		name: "s2",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return &model.MovieMeta{Title: "s2"}, true, nil
		},
	}
	g := NewGroup([]ISearcher{s1, s2})
	num := mustParseNumber(t, "ABC-123")
	meta, found, err := g.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "s2", meta.Title)
}

func TestGroupSearch_AllErrorReturnsLastError(t *testing.T) {
	s1 := &mockSearcher{
		name: "s1",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return nil, false, errors.New("s1 error")
		},
	}
	g := NewGroup([]ISearcher{s1})
	num := mustParseNumber(t, "ABC-123")
	_, found, err := g.Search(context.Background(), num)
	require.Error(t, err)
	require.False(t, found)
}

func TestGroupSearch_NotFound(t *testing.T) {
	s1 := &mockSearcher{
		name: "s1",
		searchFn: func(_ context.Context, _ *number.Number) (*model.MovieMeta, bool, error) {
			return nil, false, nil
		},
	}
	g := NewGroup([]ISearcher{s1})
	num := mustParseNumber(t, "ABC-123")
	_, found, err := g.Search(context.Background(), num)
	require.NoError(t, err)
	require.False(t, found)
}
