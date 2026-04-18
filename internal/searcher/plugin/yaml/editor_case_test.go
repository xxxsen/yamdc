package yaml

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugCase_Success(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">MyTitle</h1><div class="actors"><span>Alice</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:    "MyTitle",
			ActorSet: []string{"Alice"},
			TagSet:   nil,
			Status:   "success",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestDebugCase_ExpectError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, "not found"), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Status: "error",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestDebugCase_UnexpectedError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(404, "not found"), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Status: "success",
		},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

func TestDebugCase_TitleMismatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">Wrong</h1></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:  "Expected",
			Status: "success",
		},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

func TestDebugCase_TagSetMismatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="genres"><span>Action</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpecWithGenres("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:  "T",
			TagSet: []string{"Drama"},
			Status: "success",
		},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

func TestDebugCase_StatusNotFound(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body></body></html>`), nil
	}}
	spec := simpleOneStepSpecRequired("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:   "test",
		Input:  "ABC-123",
		Output: CaseOutput{Status: "not_found"},
	})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestDebugCase_StatusMismatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body></body></html>`), nil
	}}
	spec := simpleOneStepSpecRequired("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:   "test",
		Input:  "ABC-123",
		Output: CaseOutput{Status: "success"},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

// --- DebugRequest with multi_request ---

func TestDebugCase_ActorSetMatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="actors"><span>Alice</span><span>Bob</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:    "T",
			ActorSet: []string{"Bob", "Alice"},
			Status:   "success",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}

func TestDebugCase_ActorSetMismatch(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, `<html><body><h1 class="title">T</h1><div class="actors"><span>Alice</span></div></body></html>`), nil
	}}
	spec := simpleOneStepSpec("https://example.com")
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{
		Name:  "test",
		Input: "ABC-123",
		Output: CaseOutput{
			Title:    "T",
			ActorSet: []string{"Charlie"},
			Status:   "success",
		},
	})
	require.NoError(t, err)
	assert.False(t, result.Pass)
}

// --- debugScrapeDecodeFields with JSON error ---

func TestDebugCase_CompileError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{Input: "ABC-123"})
	require.NoError(t, err)
	assert.False(t, result.Pass)
	assert.NotEmpty(t, result.Errmsg)
}

func TestDebugCase_CompileError_ExpectError(t *testing.T) {
	cli := &testHTTPClient{roundTrip: func(_ *http.Request) (*http.Response, error) {
		return makeResponse(200, "ok"), nil
	}}
	spec := &PluginSpec{Version: 1, Name: "test", Type: "bad-type", Hosts: []string{"https://example.com"}}
	result, err := DebugCase(context.Background(), cli, spec, CaseSpec{Input: "ABC-123", Output: CaseOutput{Status: "error"}})
	require.NoError(t, err)
	assert.True(t, result.Pass)
}
