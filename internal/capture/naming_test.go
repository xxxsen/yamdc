package capture

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testSt struct {
	acts   []string
	result string
}

func TestName(t *testing.T) {
	testList := []testSt{
		{
			acts:   []string{"hello", "world"},
			result: "hello,world",
		},
		{
			acts:   []string{},
			result: defaultNonActorName,
		},
		{
			acts:   []string{"1", "2", "3", "4", "5"},
			result: defaultMultiActorAsName,
		},
	}
	for _, item := range testList {
		name := buildAuthorsName(item.acts)
		assert.Equal(t, item.result, name)
	}
}
