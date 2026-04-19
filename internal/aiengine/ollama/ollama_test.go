package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/aiengine"
)

type httpClientMock struct {
	client *http.Client
}

func (m *httpClientMock) Do(req *http.Request) (*http.Response, error) {
	return m.client.Do(req) //nolint:gosec // URL is configured by operator
}

type errHTTPClient struct{}

func (errHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("injected transport failure")
}

func TestOllamaEngine_Name(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{Response: "x", Done: true})
	}))
	t.Cleanup(srv.Close)

	eng, err := New(WithHost(srv.URL), WithModel("llama3"), WithHTTPClient(&httpClientMock{client: srv.Client()}))
	require.NoError(t, err)
	assert.Equal(t, defaultOllamaEngineName, eng.Name())
}

func TestNew_ValidationErrors(t *testing.T) {
	_, err := New(WithHost(""), WithModel("m"))
	require.ErrorIs(t, err, errOllamaHostEmpty)

	_, err = New(WithHost("http://x"), WithModel(""))
	require.ErrorIs(t, err, errOllamaModelEmpty)
}

func TestNew_DefaultHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{Response: "via-default-client", Done: true})
	}))
	t.Cleanup(srv.Close)

	// Omit WithHTTPClient to exercise newOllamaEngine default client branch.
	eng, err := New(WithHost(srv.URL), WithModel("llama3"))
	require.NoError(t, err)
	out, err := eng.Complete(context.Background(), "p", nil)
	require.NoError(t, err)
	assert.Equal(t, "via-default-client", out)
}

func TestComplete_Success(t *testing.T) {
	var gotReq Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("unexpected content type: %s", ct)
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		require.NoError(t, json.NewEncoder(w).Encode(Response{
			Response: "hi there",
			Done:     true,
		}))
	}))
	t.Cleanup(srv.Close)

	eng, err := New(WithHost(srv.URL), WithModel("llama3"), WithHTTPClient(&httpClientMock{client: srv.Client()}))
	require.NoError(t, err)
	res, err := eng.Complete(context.Background(), "hello {NAME}", map[string]any{"NAME": "ollama"})
	require.NoError(t, err)
	assert.Equal(t, "hi there", res)
	assert.Equal(t, "llama3", gotReq.Model)
	assert.Equal(t, "hello ollama", gotReq.Prompt)
	assert.False(t, gotReq.Stream)
}

func TestComplete_HostTrimsTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(Response{Response: "ok", Done: true})
	}))
	t.Cleanup(srv.Close)

	eng, err := New(WithHost(srv.URL+"/"), WithModel("m"), WithHTTPClient(&httpClientMock{client: srv.Client()}))
	require.NoError(t, err)
	out, err := eng.Complete(context.Background(), "p", nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", out)
}

func TestOllamaCompleteHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	eng, err := New(WithHost(srv.URL), WithModel("llama3"), WithHTTPClient(&httpClientMock{client: srv.Client()}))
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "hello", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, errOllamaResponseErr)
}

func TestOllamaCompleteResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{Error: "some error"})
	}))
	t.Cleanup(srv.Close)
	eng, err := New(WithHost(srv.URL), WithModel("llama3"), WithHTTPClient(&httpClientMock{client: srv.Client()}))
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "hello", nil)
	require.Error(t, err)
}

func TestOllamaCompleteEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{Response: "", Done: true})
	}))
	t.Cleanup(srv.Close)
	eng, err := New(WithHost(srv.URL), WithModel("llama3"), WithHTTPClient(&httpClientMock{client: srv.Client()}))
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "hello", nil)
	require.ErrorIs(t, err, errOllamaNoResult)
}

func TestOllamaComplete_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	t.Cleanup(srv.Close)
	eng, err := New(WithHost(srv.URL), WithModel("llama3"), WithHTTPClient(&httpClientMock{client: srv.Client()}))
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "hello", nil)
	require.ErrorContains(t, err, "decode ollama response")
}

func TestOllamaComplete_DoError(t *testing.T) {
	eng, err := New(WithHost("http://unused.example"), WithModel("m"), WithHTTPClient(errHTTPClient{}))
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "hello", nil)
	require.ErrorContains(t, err, "ollama http request failed")
}

func TestApplyOpts_DefaultHost(t *testing.T) {
	c := applyOpts()
	assert.Equal(t, "http://127.0.0.1:11434", c.Host)
	c2 := applyOpts(WithHost("h"), WithModel("m"))
	assert.Equal(t, "h", c2.Host)
	assert.Equal(t, "m", c2.Model)
}

func TestCreateOllamaEngine_viaAIEngineCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{Response: "from-create", Done: true})
	}))
	t.Cleanup(srv.Close)

	eng, err := aiengine.Create("ollama", map[string]any{
		"host":  srv.URL,
		"model": "llama3",
	}, aiengine.WithHTTPClient(&httpClientMock{client: srv.Client()}))
	require.NoError(t, err)
	out, err := eng.Complete(context.Background(), "p", nil)
	require.NoError(t, err)
	assert.Equal(t, "from-create", out)
}

func TestCreateOllamaEngine_InvalidArgs(t *testing.T) {
	_, err := aiengine.Create("ollama", make(chan int))
	require.ErrorContains(t, err, "parse ollama config")
}

func TestCreateOllamaEngine_InvalidEngineConfig(t *testing.T) {
	_, err := aiengine.Create("ollama", map[string]any{
		"host":  "",
		"model": "m",
	})
	require.ErrorIs(t, err, errOllamaHostEmpty)
}
