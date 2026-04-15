package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_FieldsAccessible(t *testing.T) {
	c := Config{
		RemoteURL: "ws://host:9222",
		DataDir:   "/tmp/data",
		Proxy:     "http://proxy:8080",
	}
	assert.Equal(t, "ws://host:9222", c.RemoteURL)
	assert.Equal(t, "/tmp/data", c.DataDir)
	assert.Equal(t, "http://proxy:8080", c.Proxy)
}

func TestConfig_ZeroValue(t *testing.T) {
	var c Config
	assert.Empty(t, c.RemoteURL)
	assert.Empty(t, c.DataDir)
	assert.Empty(t, c.Proxy)
}
