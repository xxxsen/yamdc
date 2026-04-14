package browser_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xxxsen/yamdc/internal/browser"
)

var (
	testRemoteURL = os.Getenv("BROWSER_REMOTE_URL") // e.g. "localhost:9222"
)

func newNavigator(t *testing.T) browser.INavigator {
	t.Helper()
	var nav browser.INavigator
	if testRemoteURL != "" {
		t.Logf("Using remote browser at %s", testRemoteURL)
		nav = browser.NewRemoteNavigator(testRemoteURL)
	} else {
		nav = browser.NewRodNavigator("/tmp", "")
	}
	t.Cleanup(func() { _ = nav.Close() })
	return nav
}

// TestBrowserDynamicRender verifies that the headless browser can render
// JS-generated content. quotes.toscrape.com/js/ serves quotes entirely
// via JavaScript — the raw HTML contains zero quote elements.
func TestBrowserDynamicRender(t *testing.T) {
	t.Skip("disabled")
	nav := newNavigator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := nav.Navigate(ctx, "https://quotes.toscrape.com/js/", &browser.Params{
		WaitSelector: `//div[@class="quote"]`,
		WaitTimeout:  15 * time.Second,
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	content := string(result.HTML)
	count := strings.Count(content, `class="quote"`)
	t.Logf("HTML length: %d bytes, JS-rendered quotes: %d, cookies: %d", len(result.HTML), count, len(result.Cookies))
	if count == 0 {
		t.Fatal("expected JS-rendered quotes, got none — JS did not execute")
	}
}

// TestBrowserDelayedRender verifies waiting for elements that appear after
// an artificial delay (quotes.toscrape.com/js-delayed/ has ~10s delay).
func TestBrowserDelayedRender(t *testing.T) {
	t.Skip("disabled")
	nav := newNavigator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := nav.Navigate(ctx, "https://quotes.toscrape.com/js-delayed/", &browser.Params{
		WaitSelector: `//div[@class="quote"]`,
		WaitTimeout:  20 * time.Second,
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	content := string(result.HTML)
	count := strings.Count(content, `class="quote"`)
	t.Logf("HTML length: %d bytes, delayed quotes: %d, cookies: %d", len(result.HTML), count, len(result.Cookies))
	if count == 0 {
		t.Fatal("expected delayed JS-rendered quotes, got none")
	}
}

// TestBrowserWaitXPath is a user-configurable test that accepts URL, XPath,
// and timeout via environment variables. Skipped when URL is not set.
//
//	BROWSER_TEST_URL              target URL
//	BROWSER_TEST_WAIT_SELECTOR    XPath to wait for
//	BROWSER_TEST_WAIT_TIMEOUT     timeout in seconds (default 30)
func TestBrowserWaitXPath(t *testing.T) {
	testURL := os.Getenv("BROWSER_TEST_URL")
	testWaitSelector := os.Getenv("BROWSER_TEST_WAIT_SELECTOR")
	testWaitTimeout := os.Getenv("BROWSER_TEST_WAIT_TIMEOUT")
	if testURL == "" {
		t.Skip("BROWSER_TEST_URL not set, skipping")
	}
	if testWaitSelector == "" {
		t.Skip("BROWSER_TEST_WAIT_SELECTOR not set, skipping")
	}

	waitTimeout := 30 * time.Second
	if testWaitTimeout != "" {
		if d, err := time.ParseDuration(testWaitTimeout + "s"); err == nil {
			waitTimeout = d
		}
	}

	nav := newNavigator(t)
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout+60*time.Second)
	defer cancel()

	t.Logf("URL:           %s", testURL)
	t.Logf("Wait XPath:    %s", testWaitSelector)
	t.Logf("Wait Timeout:  %s", waitTimeout)

	result, err := nav.Navigate(ctx, testURL, &browser.Params{
		WaitSelector: testWaitSelector,
		WaitTimeout:  waitTimeout,
	})
	if err != nil {
		t.Fatalf("navigate with wait xpath failed: %v", err)
	}

	t.Logf("HTML length: %d bytes, cookies: %d", len(result.HTML), len(result.Cookies))
	preview := string(result.HTML)
	if len(preview) > 3000 {
		preview = preview[:3000] + "\n... (truncated)"
	}
	fmt.Printf("--- HTML preview (with XPath wait) ---\n%s\n", preview)
}

func TestBrowserInjectHeadersAndCookies(t *testing.T) {
	var mu sync.Mutex
	var captured http.Header
	var capturedCookies []*http.Cookie

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		mu.Lock()
		captured = r.Header.Clone()
		capturedCookies = r.Cookies()
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><div id="ok">done</div></body></html>`)
	}))
	defer srv.Close()

	nav := newNavigator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := &browser.Params{
		WaitSelector: `//*[@id="ok"]`,
		WaitTimeout:  5 * time.Second,
		Headers: http.Header{
			"X-Custom-Token":  []string{"secret-123"},
			"X-Custom-Source": []string{"yamdc-test"},
		},
		Cookies: []*http.Cookie{
			{Name: "session_id", Value: "abc-xyz"},
			{Name: "theme", Value: "dark"},
		},
	}

	result, err := nav.Navigate(ctx, srv.URL, params)
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if !strings.Contains(string(result.HTML), "done") {
		t.Fatal("page content not rendered")
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify custom headers
	if got := captured.Get("X-Custom-Token"); got != "secret-123" {
		t.Errorf("X-Custom-Token: want %q, got %q", "secret-123", got)
	}
	if got := captured.Get("X-Custom-Source"); got != "yamdc-test" {
		t.Errorf("X-Custom-Source: want %q, got %q", "yamdc-test", got)
	}

	// Verify cookies
	cookieMap := make(map[string]string, len(capturedCookies))
	for _, c := range capturedCookies {
		cookieMap[c.Name] = c.Value
	}
	if v, ok := cookieMap["session_id"]; !ok || v != "abc-xyz" {
		t.Errorf("cookie session_id: want %q, got %q (exists=%v)", "abc-xyz", v, ok)
	}
	if v, ok := cookieMap["theme"]; !ok || v != "dark" {
		t.Errorf("cookie theme: want %q, got %q (exists=%v)", "dark", v, ok)
	}

	t.Logf("Headers received: %v", captured)
	t.Logf("Cookies received: %v", cookieMap)
}
