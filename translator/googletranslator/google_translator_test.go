package googletranslator

import (
	"context"
	"testing"
	"yamdc/translator"

	"github.com/stretchr/testify/assert"
)

func TestTranslate(t *testing.T) {
	impl, err := New()
	assert.NoError(t, err)
	translator.SetTranslator(impl)
	assert.NoError(t, err)
	res, err := translator.Translate(context.Background(), "hello world", "auto", "zh")
	assert.NoError(t, err)
	t.Logf("result:%s", res)
}
