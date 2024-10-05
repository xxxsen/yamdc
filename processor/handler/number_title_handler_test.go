package handler

import (
	"context"
	"testing"
	"yamdc/model"
	"yamdc/number"

	"github.com/stretchr/testify/assert"
)

type testSt struct {
	in     string
	number string
	out    string
}

func TestNumberTitle(t *testing.T) {
	lst := []testSt{
		{
			in:     "you see this number:zzz_123?",
			number: "zzz-123",
			out:    "you see this number:zzz_123?",
		},
		{
			in:     "hello world",
			number: "zzz-123",
			out:    "ZZZ-123 hello world",
		},
		{
			in:     "hahahaha aaa-1234",
			number: "aaa-1234",
			out:    "hahahaha aaa-1234",
		},
		{
			in:     "",
			number: "zzz-123",
			out:    "ZZZ-123 ",
		},
		{
			in:     "zzz 232 hahaha",
			number: "zzz_232",
			out:    "zzz 232 hahaha",
		},
	}
	h, err := CreateHandler(HNumberTitle, nil)
	assert.NoError(t, err)
	for _, tst := range lst {
		nid, err := number.Parse(tst.number)
		assert.NoError(t, err)
		meta := &model.FileContext{
			Meta: &model.AvMeta{
				Number: tst.number,
				Title:  tst.in,
			},
			Number: nid,
		}
		err = h.Handle(context.Background(), meta)
		assert.NoError(t, err)
		assert.Equal(t, tst.out, meta.Meta.Title)
	}
}
