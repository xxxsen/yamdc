package google

import (
	"context"
	"testing"
	"github.com/xxxsen/yamdc/internal/translator"

	"github.com/stretchr/testify/assert"
)

func TestTranslate(t *testing.T) {
	impl := New()
	translator.SetTranslator(impl)
	res, err := translator.Translate(context.Background(), "hello world", "auto", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
}
