package yaml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorReader struct{}

func (r *errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

func TestRequestDebug_WithBody(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", bytes.NewReader([]byte("body-data")))
	dbg := requestDebug(req)
	assert.Equal(t, "body-data", dbg.Body)
}

// --- traceAssignStringField ---

func TestCaptureHTTPResponse(t *testing.T) {
	rsp := makeResponse(200, "body-text")
	defer func() { _ = rsp.Body.Close() }()
	resp, err := captureHTTPResponse(rsp, "")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "body-text", resp.Body)
}

// --- cloneHeader ---

func TestCloneHeader(t *testing.T) {
	h := http.Header{"X-Key": {"v1", "v2"}}
	c := cloneHeader(h)
	assert.Equal(t, []string{"v1", "v2"}, c["X-Key"])
}

// --- helpers ---

func TestCaptureHTTPResponse_DecodeError(t *testing.T) {
	rsp := makeResponse(200, "hello")
	defer func() { _ = rsp.Body.Close() }()
	_, err := captureHTTPResponse(rsp, "unknown-charset")
	require.Error(t, err)
}

// --- previewBody truncation ---

func TestPreviewBody_Truncation(t *testing.T) {
	body := strings.Repeat("a", 5000)
	result := previewBody(body)
	assert.Len(t, result, 4000)
}

// --- debugWorkflowSearchSelect nil baseResp ---

func TestCaptureHTTPResponse_ReadError(t *testing.T) {
	rsp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(&errorReader{}),
		Header:     make(http.Header),
	}
	_, err := captureHTTPResponse(rsp, "")
	require.Error(t, err)
}

func TestRequestDebug_NilBody(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	dbg := requestDebug(req)
	assert.Empty(t, dbg.Body)
}

func TestNormalizeStringSet_WithDuplicatesAndEmpty(t *testing.T) {
	result := normalizeStringSet([]string{"b", "", "a", " ", "b", "c"})
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestEqualNormalizedSet_DifferentLengths(t *testing.T) {
	assert.False(t, equalNormalizedSet([]string{"a"}, []string{"a", "b"}))
}

func TestEqualNormalizedSet_DifferentContent(t *testing.T) {
	assert.False(t, equalNormalizedSet([]string{"a", "b"}, []string{"a", "c"}))
}

func TestRenderCondition_NilCondition(t *testing.T) {
	assert.Empty(t, renderCondition(nil))
}

func TestRenderCondition_Named(t *testing.T) {
	cond := &compiledCondition{name: "contains"}
	assert.Equal(t, "contains", renderCondition(cond))
}

func TestPreviewBody(t *testing.T) {
	short := "hello"
	assert.Equal(t, short, previewBody(short))

	long := strings.Repeat("a", 5000)
	assert.Len(t, previewBody(long), 4000)
}

func TestEqualNormalizedSet(t *testing.T) {
	assert.True(t, equalNormalizedSet([]string{"a", "b"}, []string{"b", "a"}))
	assert.False(t, equalNormalizedSet([]string{"a"}, []string{"b"}))
	assert.False(t, equalNormalizedSet([]string{"a", "b"}, []string{"a"}))
	assert.True(t, equalNormalizedSet([]string{"a", "a"}, []string{"a"}))
}

func TestNormalizeStringSet(t *testing.T) {
	result := normalizeStringSet([]string{"b", " a ", "", "b"})
	assert.Equal(t, []string{"a", "b"}, result)
}

func TestRenderCondition(t *testing.T) {
	assert.Equal(t, "", renderCondition(nil))
	assert.Equal(t, "contains", renderCondition(&compiledCondition{name: "contains"}))
}
