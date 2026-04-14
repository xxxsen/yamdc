package browser_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xxxsen/yamdc/internal/browser"
)

var (
	testURL          = getOrDefault("BROWSER_TEST_URL", "https://learngitbranching.js.org/")
	testWaitSelector = getOrDefault("BROWSER_TEST_WAIT_SELECTOR", `//div[contains(@class,"terminal-window")]`)
	testWaitTimeout  = getOrDefault("BROWSER_TEST_WAIT_TIMEOUT", "30")
)

func getOrDefault(key string, value string) string {
	v := os.Getenv(key)
	if len(v) == 0 {
		return value
	}
	return v
}

func newNavigator(t *testing.T) browser.INavigator {
	t.Helper()
	nav := browser.NewRodNavigator("/tmp", "")
	t.Cleanup(func() { _ = nav.Close() })
	return nav
}

func parseWaitTimeout() time.Duration {
	if testWaitTimeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(testWaitTimeout + "s")
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// TestBrowserDynamicRender verifies that the headless browser can render
// JS-generated content. It navigates to the URL without a wait selector
// and checks that the returned HTML contains content that only exists
// after JavaScript execution.
func TestBrowserDynamicRender(t *testing.T) {
	if testURL == "" {
		t.Skip("BROWSER_TEST_URL not set, skipping")
	}

	nav := newNavigator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	html, err := nav.Navigate(ctx, testURL, &browser.Params{})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if len(html) == 0 {
		t.Fatal("returned HTML is empty")
	}

	content := string(html)
	t.Logf("HTML length: %d bytes", len(html))

	staticOnly := strings.Contains(content, `<div id="app"></div>`) &&
		!strings.Contains(content, `<canvas`) &&
		!strings.Contains(content, `class="react"`)
	if staticOnly {
		t.Error("HTML appears to be static (un-rendered) shell; JS did not execute")
	}

	preview := content
	if len(preview) > 3000 {
		preview = preview[:3000] + "\n... (truncated)"
	}
	fmt.Printf("--- Rendered HTML preview ---\n%s\n", preview)
}

// TestBrowserWaitXPath verifies that ElementX can find a specific XPath
// in a JS-rendered page. Uses BROWSER_TEST_WAIT_SELECTOR env var.
func TestBrowserWaitXPath(t *testing.T) {
	if testURL == "" {
		t.Skip("BROWSER_TEST_URL not set, skipping")
	}
	if testWaitSelector == "" {
		t.Skip("BROWSER_TEST_WAIT_SELECTOR not set, skipping")
	}

	nav := newNavigator(t)
	waitTimeout := parseWaitTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout+60*time.Second)
	defer cancel()

	params := &browser.Params{
		WaitSelector: testWaitSelector,
		WaitTimeout:  waitTimeout,
	}

	t.Logf("URL:           %s", testURL)
	t.Logf("Wait XPath:    %s", testWaitSelector)
	t.Logf("Wait Timeout:  %s", waitTimeout)

	html, err := nav.Navigate(ctx, testURL, params)
	if err != nil {
		t.Fatalf("navigate with wait xpath failed: %v", err)
	}

	t.Logf("HTML length: %d bytes", len(html))
	preview := string(html)
	if len(preview) > 3000 {
		preview = preview[:3000] + "\n... (truncated)"
	}
	fmt.Printf("--- HTML preview (with XPath wait) ---\n%s\n", preview)
}
