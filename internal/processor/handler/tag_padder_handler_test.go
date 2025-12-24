package handler

import (
	"context"
	"testing"
	"yamdc/internal/model"
	"yamdc/internal/number"

	"github.com/stretchr/testify/assert"
)

type testPair struct {
	in        string
	tagCount  int
	existTags []string
}

func TestTagPadde(t *testing.T) {
	tsts := []testPair{
		{
			in:        "fc2-1234-c-4k",
			tagCount:  4,
			existTags: []string{},
		},
		{
			in:        "fc2-ppv-123",
			tagCount:  2,
			existTags: []string{"FC2"},
		},
		{
			in:        "heyzo-123",
			tagCount:  2,
			existTags: []string{"HEYZO"},
		},
		{
			in:        "111111-11",
			tagCount:  1,
			existTags: []string{},
		},
	}
	for _, item := range tsts {
		num, err := number.Parse(item.in)
		assert.NoError(t, err)
		padder := &tagPadderHandler{}
		fc := &model.FileContext{
			Number: num,
			Meta:   &model.MovieMeta{},
		}
		padder.Handle(context.Background(), fc)
		assert.Equal(t, item.tagCount, len(fc.Meta.Genres))
		for _, existTag := range item.existTags {
			assert.Contains(t, fc.Meta.Genres, existTag)
		}
	}
}
