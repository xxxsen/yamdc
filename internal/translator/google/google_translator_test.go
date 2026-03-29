package google

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslate(t *testing.T) {
	impl := New()
	res, err := impl.Translate(context.Background(), "hello world", "auto", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
}
