package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAPI(t *testing.T) {
	api := NewAPI(nil, nil, nil, "/tmp/save", nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, api)
	assert.Equal(t, "/tmp/save", api.saveDir)
}
