package face

import (
	"context"
	"errors"
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRec struct {
	name string
	recs []image.Rectangle
	err  error
}

func (m *mockRec) Name() string { return m.name }

func (m *mockRec) SearchFaces(ctx context.Context, data []byte) ([]image.Rectangle, error) {
	_ = ctx
	_ = data
	return m.recs, m.err
}

func TestFindMaxFace_Empty(t *testing.T) {
	var want image.Rectangle
	got := FindMaxFace(nil)
	assert.Equal(t, want, got)
}

func TestFindMaxFace_AllZero(t *testing.T) {
	var z image.Rectangle
	got := FindMaxFace([]image.Rectangle{z, z})
	assert.Equal(t, z, got)
}

func TestFindMaxFace_PicksLargestArea(t *testing.T) {
	small := image.Rect(0, 0, 2, 2)  // 4
	med := image.Rect(1, 1, 4, 3)    // 3*2=6
	big := image.Rect(0, 0, 5, 5)    // 25
	got := FindMaxFace([]image.Rectangle{small, med, big})
	assert.Equal(t, big, got)
}

func TestGroup_Name(t *testing.T) {
	g := NewGroup(nil)
	assert.Equal(t, "group", g.Name())
}

func TestGroup_SearchFaces_FirstSuccess(t *testing.T) {
	fail := &mockRec{name: "a", err: errors.New("fail a")}
	ok := &mockRec{name: "b", recs: []image.Rectangle{image.Rect(0, 0, 2, 2)}}
	g := NewGroup([]IFaceRec{fail, ok})
	recs, err := g.SearchFaces(context.Background(), []byte("img"))
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, image.Rect(0, 0, 2, 2), recs[0])
}

func TestGroup_SearchFaces_AllFail_ReturnsLastError(t *testing.T) {
	e1 := errors.New("first")
	e2 := errors.New("last")
	g := NewGroup([]IFaceRec{
		&mockRec{name: "x", err: e1},
		&mockRec{name: "y", err: e2},
	})
	_, err := g.SearchFaces(context.Background(), []byte{})
	require.Error(t, err)
	assert.Equal(t, e2, err)
}

func TestGroup_SearchFaces_SingleImplError(t *testing.T) {
	want := errors.New("only")
	g := NewGroup([]IFaceRec{&mockRec{name: "solo", err: want}})
	_, err := g.SearchFaces(context.Background(), []byte{1, 2, 3})
	require.Error(t, err)
	assert.Equal(t, want, err)
}
