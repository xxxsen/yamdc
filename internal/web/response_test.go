package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	writeSuccess(rec, "hello", map[string]int{"x": 1})
	var resp responseBody
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "hello", resp.Message)
}

func TestWriteFailCodeZeroDefaultsToUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	writeFail(rec, 0, "something wrong")
	var resp responseBody
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, errCodeUnknown, resp.Code)
	assert.Equal(t, "something wrong", resp.Message)
}

func TestWriteFailNonZeroCode(t *testing.T) {
	rec := httptest.NewRecorder()
	writeFail(rec, 42, "custom error")
	var resp responseBody
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, 42, resp.Code)
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"k": "v"})
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}
