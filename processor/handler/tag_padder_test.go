package handler

import (
	"context"
	"testing"
	"yamdc/model"
	"yamdc/number"

	"github.com/stretchr/testify/assert"
)

func TestTagPadde(t *testing.T) {
	num, err := number.Parse("fc2-1234-c-4k")
	assert.NoError(t, err)
	padder := &tagPadder{}
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.AvMeta{},
	}
	padder.Handle(context.Background(), fc)
	assert.Equal(t, 3, len(fc.Meta.Genres))
}
