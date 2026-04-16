package flarerr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithParams_GetParams(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	assert.Nil(t, GetParams(ctx))

	params := &Params{}
	ctx = WithParams(ctx, params)
	got := GetParams(ctx)
	assert.Same(t, params, got)
}

func TestGetParams_WrongType(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxKey{}, "not-a-params")
	assert.Nil(t, GetParams(ctx))
}
