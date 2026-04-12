package yaml

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

type mockClient struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	if m.roundTrip != nil {
		return m.roundTrip(req)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func TestEditorContextNumberPropagation(t *testing.T) {
	// mock yaml plugin spec
	rawSpec := &PluginSpec{
		Version: 1,
		Name:    "test_propagation",
		Type:    "two-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "GET",
			Path:   "/search/${number}", // Expecting the context number to be populated here
			AcceptStatusCodes: []int{200},
			Response: &ResponseSpec{
				DecodeCharset: "utf-8",
			},
		},
		Workflow: &WorkflowSpec{
			SearchSelect: &SearchSelectWorkflowSpec{
				Selectors: []*SelectorListSpec{
					{Name: "link", Kind: "xpath", Expr: "//a/@href"},
				},
				Return: "${item.link}",
				NextRequest: &RequestSpec{
					Method: "GET",
					Path:   "/detail/${number}",
					AcceptStatusCodes: []int{200},
				},
			},
		},
		Scrape: &ScrapeSpec{
			Format: "html",
			Fields: map[string]*FieldSpec{
				"title": {
					Selector: &SelectorSpec{
						Kind: "xpath",
						Expr: "//title/text()",
					},
					Parser: ParserSpec{
						Kind: "string",
					},
				},
			},
		},
	}

	cli := &mockClient{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			body := "<html><title>Test: " + req.URL.Path + "</title><body><a href=\"/link\">link</a></body></html>"
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte(body))),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		},
	}

	numberToTest := "TEST-A-123"

	t.Run("DebugRequest", func(t *testing.T) {
		res, err := DebugRequest(context.Background(), cli, rawSpec, numberToTest)
		if err != nil {
			t.Fatalf("DebugRequest error: %v", err)
		}
		if res.Request.URL != "https://example.com/search/TEST-A-123" {
			t.Errorf("expected URL https://example.com/search/TEST-A-123, got: %s", res.Request.URL)
		}
	})

	t.Run("DebugScrape", func(t *testing.T) {
		res, err := DebugScrape(context.Background(), cli, rawSpec, numberToTest)
		if err != nil {
			t.Fatalf("DebugScrape error: %v", err)
		}
		if res.Error != "" {
			t.Fatalf("DebugScrape returned error field: %s", res.Error)
		}
		if res.Request.URL != "https://example.com/detail/TEST-A-123" {
			t.Errorf("expected URL https://example.com/detail/TEST-A-123, got: %s", res.Request.URL)
		}
		// Confirm title field captured the populated URL
		f, ok := res.Fields["title"]
		if !ok || len(f.SelectorValues) == 0 || f.SelectorValues[0] != "Test: /detail/TEST-A-123" {
			t.Errorf("expected title 'Test: /detail/TEST-A-123', got: %+v", f)
		}
	})

	t.Run("DebugWorkflow", func(t *testing.T) {
		res, err := DebugWorkflow(context.Background(), cli, rawSpec, numberToTest)
		if err != nil {
			t.Fatalf("DebugWorkflow error: %v", err)
		}
		if res.Error != "" {
			t.Fatalf("DebugWorkflow returned error field: %s", res.Error)
		}
		if len(res.Steps) == 0 {
			t.Fatalf("DebugWorkflow returned 0 steps")
		}
		req := res.Steps[0].Request
		if req == nil {
			t.Fatalf("Step 0 request is nil")
		}
		if req.URL != "https://example.com/search/TEST-A-123" {
			t.Errorf("expected URL https://example.com/search/TEST-A-123, got: %s", req.URL)
		}
	})
}
