package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/client"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

// geminiHTTPClientToLocal rewrites the outbound Gemini API URL to a local httptest server.
func geminiHTTPClientToLocal(t *testing.T, srv *httptest.Server) client.IHTTPClient {
	t.Helper()
	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = baseURL.Scheme
		req.URL.Host = baseURL.Host
		return srv.Client().Do(req) //nolint:gosec
	})
}

func TestNew_MissingKeyOrModel(t *testing.T) {
	_, err := New(WithKey(""), WithModel("m"))
	require.ErrorIs(t, err, errKeyEmpty)

	_, err = New(WithKey("k"), WithModel(""))
	require.ErrorIs(t, err, errModelEmpty)
}

func TestNew_DefaultHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{
			Candidates: []Candidate{
				{Content: Content{Parts: []Part{{Text: "ok"}}}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	eng, err := New(WithKey("k"), WithModel("any"), WithHTTPClient(geminiHTTPClientToLocal(t, srv)))
	require.NoError(t, err)
	assert.Equal(t, defaultGeminiEngineName, eng.Name())
}

func TestComplete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.URL.Path, "/models/test-model:generateContent")
		assert.Contains(t, r.URL.RawQuery, "key=test-key")

		var body Request
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Len(t, body.Contents, 1)
		assert.Equal(t, "hello world", body.Contents[0].Parts[0].Text)
		assert.Equal(t, 0.0, body.GenerationConfig.Temperature)

		_ = json.NewEncoder(w).Encode(Response{
			Candidates: []Candidate{
				{
					Content:      Content{Parts: []Part{{Text: "  translated  "}}},
					FinishReason: "STOP",
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	eng, err := newGeminiEngine(&config{
		Key:        "test-key",
		Model:      "test-model",
		HTTPClient: geminiHTTPClientToLocal(t, srv),
	})
	require.NoError(t, err)

	out, err := eng.Complete(context.Background(), "hello {X}", map[string]interface{}{"X": "world"})
	require.NoError(t, err)
	assert.Equal(t, "translated", out)
}

func TestComplete_HTTPStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	eng, err := newGeminiEngine(&config{
		Key:        "k",
		Model:      "m",
		HTTPClient: geminiHTTPClientToLocal(t, srv),
	})
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "p", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, errGeminiResponseErr)
}

func TestComplete_DoError(t *testing.T) {
	eng, err := newGeminiEngine(&config{
		Key:   "k",
		Model: "m",
		HTTPClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}),
	})
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "p", nil)
	require.ErrorContains(t, err, "execute http request")
}

func TestComplete_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	t.Cleanup(srv.Close)

	eng, err := newGeminiEngine(&config{
		Key:        "k",
		Model:      "m",
		HTTPClient: geminiHTTPClientToLocal(t, srv),
	})
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "p", nil)
	require.ErrorContains(t, err, "decode response")
}

func TestComplete_NoCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{
			PromptFeedback: PromptFeedback{BlockReason: "SAFETY"},
			Candidates:     []Candidate{},
		})
	}))
	t.Cleanup(srv.Close)

	eng, err := newGeminiEngine(&config{
		Key:        "k",
		Model:      "m",
		HTTPClient: geminiHTTPClientToLocal(t, srv),
	})
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "p", nil)
	require.ErrorIs(t, err, errNoTranslateResult)
	require.Contains(t, err.Error(), "SAFETY")
}

func TestComplete_NoParts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{
			Candidates: []Candidate{
				{Content: Content{Parts: []Part{}}, FinishReason: "OTHER"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	eng, err := newGeminiEngine(&config{
		Key:        "k",
		Model:      "m",
		HTTPClient: geminiHTTPClientToLocal(t, srv),
	})
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "p", nil)
	require.ErrorIs(t, err, errNoTranslateResultPart)
}

func TestComplete_EmptyText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{
			Candidates: []Candidate{
				{Content: Content{Parts: []Part{{Text: "  \t "}}}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	eng, err := newGeminiEngine(&config{
		Key:        "k",
		Model:      "m",
		HTTPClient: geminiHTTPClientToLocal(t, srv),
	})
	require.NoError(t, err)
	_, err = eng.Complete(context.Background(), "p", nil)
	require.ErrorIs(t, err, errNoTranslateResultText)
}

func TestBuildRequest_EdgeCases(t *testing.T) {
	r := buildRequest("plain", nil)
	require.Len(t, r.Contents, 1)
	assert.Equal(t, "plain", r.Contents[0].Parts[0].Text)

	r2 := buildRequest("a {K} b", map[string]interface{}{"K": "v"})
	assert.Equal(t, "a v b", r2.Contents[0].Parts[0].Text)
}

func TestApplyOpts_DefaultModel(t *testing.T) {
	c := applyOpts()
	assert.Equal(t, "gemini-2.0-flash", c.Model)
	c2 := applyOpts(WithModel("x"), WithKey("k"))
	assert.Equal(t, "x", c2.Model)
	assert.Equal(t, "k", c2.Key)
}

func TestCreateGeminiEngine_viaAIEngineCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{
			Candidates: []Candidate{
				{Content: Content{Parts: []Part{{Text: "from-create"}}}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	eng, err := aiengine.Create("gemini", map[string]interface{}{
		"key":   "k",
		"model": "m",
	}, aiengine.WithHTTPClient(geminiHTTPClientToLocal(t, srv)))
	require.NoError(t, err)
	out, err := eng.Complete(context.Background(), "hi", nil)
	require.NoError(t, err)
	assert.Equal(t, "from-create", out)
}

func TestCreateGeminiEngine_InvalidArgs(t *testing.T) {
	_, err := aiengine.Create("gemini", make(chan int))
	require.ErrorContains(t, err, "convert gemini config")
}

func TestNewGeminiEngine_DefaultHTTPClient_Used(t *testing.T) {
	eng, err := newGeminiEngine(&config{
		Key:   "k",
		Model: "m",
	})
	require.NoError(t, err)
	assert.NotNil(t, eng.c.HTTPClient)
}

func TestCreateGeminiEngine_WithoutHTTPClient(t *testing.T) {
	eng, err := aiengine.Create("gemini", map[string]interface{}{
		"key":   "k",
		"model": "m",
	})
	require.NoError(t, err)
	assert.NotNil(t, eng)
}

func TestComplete_WithHTTPClientOption(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{
			Candidates: []Candidate{
				{Content: Content{Parts: []Part{{Text: "from-option-client"}}}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	eng, err := createGeminiEngine(
		map[string]interface{}{"key": "k", "model": "m"},
		aiengine.WithHTTPClient(geminiHTTPClientToLocal(t, srv)),
	)
	require.NoError(t, err)
	out, err := eng.Complete(context.Background(), "hi", nil)
	require.NoError(t, err)
	assert.Equal(t, "from-option-client", out)
}
