package web

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleHealthz(t *testing.T) {
	api := &API{}
	c, rec := newGinContext(http.MethodGet, "/api/healthz", nil)
	api.handleHealthz(c)
	resp := decodeResponse(t, rec)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "ok", resp.Message)
}
