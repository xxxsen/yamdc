package translator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/translator"
)

type stubTr struct {
	name string
	out  string
	err  error
}

func (s *stubTr) Name() string { return s.name }

func (s *stubTr) Translate(_ context.Context, wording, _, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.out, nil
}

func TestNewGroup_Name(t *testing.T) {
	g := translator.NewGroup(
		&stubTr{name: "a", out: "x"},
		&stubTr{name: "b", out: "y"},
	)
	assert.Equal(t, "G:[a,b]", g.Name())
}

func TestNewGroup_EmptyName(t *testing.T) {
	g := translator.NewGroup()
	assert.Equal(t, "G:[]", g.Name())
}

func TestGroup_Translate_FirstSuccess(t *testing.T) {
	g := translator.NewGroup(
		&stubTr{name: "first", out: "ok"},
		&stubTr{name: "second", err: errors.New("skip")},
	)
	out, err := g.Translate(context.Background(), "w", "en", "zh")
	require.NoError(t, err)
	assert.Equal(t, "ok", out)
}

func TestGroup_Translate_SkipErrorThenSuccess(t *testing.T) {
	g := translator.NewGroup(
		&stubTr{name: "bad", err: errors.New("boom")},
		&stubTr{name: "good", out: "done"},
	)
	out, err := g.Translate(context.Background(), "w", "en", "zh")
	require.NoError(t, err)
	assert.Equal(t, "done", out)
}

func TestGroup_Translate_SkipEmptyThenSuccess(t *testing.T) {
	g := translator.NewGroup(
		&stubTr{name: "empty", out: ""},
		&stubTr{name: "good", out: "x"},
	)
	out, err := g.Translate(context.Background(), "w", "en", "zh")
	require.NoError(t, err)
	assert.Equal(t, "x", out)
}

func TestGroup_Translate_AllFail(t *testing.T) {
	g := translator.NewGroup(
		&stubTr{name: "a", err: errors.New("e1")},
		&stubTr{name: "b", err: errors.New("e2")},
	)
	_, err := g.Translate(context.Background(), "w", "en", "zh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "b")
}

func TestGroup_Translate_AllEmptyOrError(t *testing.T) {
	g := translator.NewGroup(
		&stubTr{name: "a", out: ""},
		&stubTr{name: "b", err: errors.New("x")},
	)
	_, err := g.Translate(context.Background(), "w", "en", "zh")
	require.Error(t, err)
}

func TestGroup_Translate_EmptyGroup(t *testing.T) {
	g := translator.NewGroup()
	out, err := g.Translate(context.Background(), "w", "en", "zh")
	assert.Empty(t, out)
	assert.NoError(t, err)
}
