package handler

import (
	"av-capture/constant"
	"av-capture/model"
	"av-capture/number"
	"context"
	"testing"

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
	assert.Contains(t, fc.Meta.Genres, constant.Tag4K)
	assert.Contains(t, fc.Meta.Genres, constant.TagChineseSubtitle)
	assert.Contains(t, fc.Meta.Genres, constant.TagUncensored)
}
