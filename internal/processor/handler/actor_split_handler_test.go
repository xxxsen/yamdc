package handler

import (
	"context"
	"testing"
	"github.com/xxxsen/yamdc/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestSplitActor(t *testing.T) {
	tests := []string{
		"永野司 (永野つかさ)",
		"萨达（AA萨达）",
	}
	h := &actorSplitHandler{}
	in := &model.FileContext{
		Meta: &model.MovieMeta{
			Actors: tests,
		},
	}
	err := h.Handle(context.Background(), in)
	assert.NoError(t, err)
	t.Logf("read actor list:%+v", in.Meta.Actors)
}
