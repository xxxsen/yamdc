package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/xxxsen/yamdc/internal/client"
)

type httpClientMock struct {
	client *http.Client
}

func (m *httpClientMock) Do(req *http.Request) (*http.Response, error) {
	return m.client.Do(req)
}

func withMockClient(t *testing.T, cli client.IHTTPClient) func() {
	t.Helper()
	old := client.DefaultClient()
	client.SetDefault(cli)
	return func() {
		client.SetDefault(old)
	}
}

func TestOllamaCompleteSuccess(t *testing.T) {
	var gotReq Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("unexpected content type: %s", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		_ = json.NewEncoder(w).Encode(Response{
			Response: "hi there",
			Done:     true,
		})
	}))
	defer srv.Close()
	restore := withMockClient(t, &httpClientMock{client: srv.Client()})
	defer restore()

	eng, err := New(WithHost(srv.URL), WithModel("llama3"))
	if err != nil {
		t.Fatalf("new engine failed: %v", err)
	}
	res, err := eng.Complete(context.Background(), "hello {NAME}", map[string]interface{}{"NAME": "ollama"})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if res != "hi there" {
		t.Fatalf("unexpected result: %s", res)
	}
	if gotReq.Model != "llama3" {
		t.Fatalf("unexpected model: %s", gotReq.Model)
	}
	if gotReq.Prompt != "hello ollama" {
		t.Fatalf("unexpected prompt: %s", gotReq.Prompt)
	}
	if gotReq.Stream {
		t.Fatalf("stream should be false")
	}
}

func TestOllamaCompleteHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	restore := withMockClient(t, &httpClientMock{client: srv.Client()})
	defer restore()

	eng, err := New(WithHost(srv.URL), WithModel("llama3"))
	if err != nil {
		t.Fatalf("new engine failed: %v", err)
	}
	if _, err := eng.Complete(context.Background(), "hello", nil); err == nil {
		t.Fatalf("expect error but got nil")
	}
}

func TestOllamaCompleteResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{
			Error: "some error",
		})
	}))
	defer srv.Close()
	restore := withMockClient(t, &httpClientMock{client: srv.Client()})
	defer restore()

	eng, err := New(WithHost(srv.URL), WithModel("llama3"))
	if err != nil {
		t.Fatalf("new engine failed: %v", err)
	}
	if _, err := eng.Complete(context.Background(), "hello", nil); err == nil {
		t.Fatalf("expect error but got nil")
	}
}

func TestOllamaCompleteEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{
			Response: "",
			Done:     true,
		})
	}))
	defer srv.Close()
	restore := withMockClient(t, &httpClientMock{client: srv.Client()})
	defer restore()

	eng, err := New(WithHost(srv.URL), WithModel("llama3"))
	if err != nil {
		t.Fatalf("new engine failed: %v", err)
	}
	if _, err := eng.Complete(context.Background(), "hello", nil); err == nil {
		t.Fatalf("expect error but got nil")
	}
}
