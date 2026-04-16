package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyEnvOverridesSearchMetaCache(t *testing.T) {
	c := SwitchConfig{EnableSearchMetaCache: true}
	t.Setenv("ENABLE_SEARCH_META_CACHE", "false")
	ApplyEnvOverrides(&c)
	assert.False(t, c.EnableSearchMetaCache)
}

func TestApplyEnvOverridesPigoFaceRecognizer(t *testing.T) {
	c := SwitchConfig{EnablePigoFaceRecognizer: true}
	t.Setenv("ENABLE_PIGO_FACE_RECOGNIZER", "false")
	ApplyEnvOverrides(&c)
	assert.False(t, c.EnablePigoFaceRecognizer)
}

func TestApplyEnvOverridesNoEnvKeepsDefaults(t *testing.T) {
	c := SwitchConfig{EnableSearchMetaCache: true, EnablePigoFaceRecognizer: true}
	ApplyEnvOverrides(&c)
	assert.True(t, c.EnableSearchMetaCache)
	assert.True(t, c.EnablePigoFaceRecognizer)
}

func TestApplyEnvOverridesNonFalseValueIgnored(t *testing.T) {
	c := SwitchConfig{EnableSearchMetaCache: true}
	t.Setenv("ENABLE_SEARCH_META_CACHE", "true")
	ApplyEnvOverrides(&c)
	assert.True(t, c.EnableSearchMetaCache)
}
