package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMove(t *testing.T) {
	err := Move("/data/1", "/data_temp/123")
	assert.NoError(t, err)
}
