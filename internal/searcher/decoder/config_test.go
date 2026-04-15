package decoder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultStringAndListParsers(t *testing.T) {
	assert.Equal(t, "a", defaultStringParser("a"))
	assert.Equal(t, []string{"a", "b"}, defaultStringListParser([]string{"a", "b"}))
	assert.Equal(t, "", defaultStringProcessor(""))
	assert.Equal(t, []string(nil), defaultStringListProcessor(nil))
}
