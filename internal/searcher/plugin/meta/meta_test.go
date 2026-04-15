package meta

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCtxKey struct{}

func TestSetGetNumberID(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, GetNumberID(ctx))

	ctx = SetNumberID(ctx, "ABC-123")
	assert.Equal(t, "ABC-123", GetNumberID(ctx))

	// child context inherits value
	child := context.WithValue(ctx, testCtxKey{}, "noise")
	assert.Equal(t, "ABC-123", GetNumberID(child))
}

func TestGetNumberID_WrongTypeIgnored(t *testing.T) {
	ctx := context.WithValue(context.Background(), defaultNumberIDKey, 42)
	assert.Empty(t, GetNumberID(ctx))
}

func TestGetNumberID_NilValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), defaultNumberIDKey, nil)
	assert.Empty(t, GetNumberID(ctx))
}
