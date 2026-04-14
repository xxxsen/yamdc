package browser

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

func NewNavigator(cfg *Config) INavigator {
	if cfg.RemoteURL != "" {
		return NewRemoteNavigator(cfg.RemoteURL)
	}
	return NewRodNavigator(cfg.DataDir, cfg.Proxy)
}

const (
	defaultPageTimeout = 60 * time.Second
	browserIdleTimeout = 30 * time.Second
)

type rodBrowser struct {
	mu          sync.Mutex
	browser     *rod.Browser
	remote      bool
	remoteURL   string
	proxy       string
	dataDir     string
	activeCount int
	idleTimer   *time.Timer
}

func NewRodNavigator(dataDir string, proxy string) INavigator {
	return &rodBrowser{
		dataDir: dataDir,
		proxy:   proxy,
	}
}

// NewRemoteNavigator connects to an existing browser via CDP remote
// debugging. The browser is NOT managed by us — no idle-close, no
// process kill on Close().
// remoteURL accepts formats like "9222", ":9222", "host:9222",
// "ws://host:9222", "http://host:9222".
func NewRemoteNavigator(remoteURL string) INavigator {
	return &rodBrowser{
		remote:    true,
		remoteURL: remoteURL,
	}
}

func (rb *rodBrowser) ensureBrowser() (*rod.Browser, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.browser != nil {
		return rb.browser, nil
	}

	var controlURL string
	rollback := func() {}
	if rb.remote {
		wsURL, err := launcher.ResolveURL(rb.remoteURL)
		if err != nil {
			return nil, fmt.Errorf("resolve remote browser URL %q failed: %w", rb.remoteURL, err)
		}
		controlURL = wsURL
	} else {
		launcher.DefaultBrowserDir = filepath.Join(rb.dataDir, "browser")
		l := launcher.New().
			Set("headless", "new").
			NoSandbox(true).
			Set("disable-gpu").
			Set("disable-dev-shm-usage").
			Set("window-size", "1920,1080")
		if rb.proxy != "" {
			l = l.Proxy(rb.proxy)
		}
		var err error
		controlURL, err = l.Launch()
		if err != nil {
			return nil, fmt.Errorf("launch browser failed: %w", err)
		}
		rollback = func() { l.Kill() }
	}

	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		rollback()
		return nil, fmt.Errorf("connect browser failed: %w", err)
	}
	rb.browser = b
	return rb.browser, nil
}

func (rb *rodBrowser) acquireNavigate() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.activeCount++
	if rb.idleTimer != nil {
		rb.idleTimer.Stop()
		rb.idleTimer = nil
	}
}

func (rb *rodBrowser) releaseNavigate(ctx context.Context) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.activeCount--
	logutil.GetLogger(ctx).Debug("browser navigate released",
		zap.Int("active_count", rb.activeCount))
	if rb.activeCount == 0 && rb.browser != nil && !rb.remote {
		rb.startIdleTimerLocked()
	}
}

func (rb *rodBrowser) Navigate(ctx context.Context, rawURL string, params *Params) (*NavigateResult, error) {
	b, err := rb.ensureBrowser()
	if err != nil {
		return nil, err
	}

	if !rb.remote {
		rb.acquireNavigate()
		defer rb.releaseNavigate(ctx)
	}

	page, err := stealth.Page(b)
	if err != nil {
		return nil, fmt.Errorf("create stealth page failed: %w", err)
	}
	defer func() { _ = page.Close() }()

	if err := injectCookies(page, rawURL, params.Cookies); err != nil {
		return nil, fmt.Errorf("inject cookies failed: %w", err)
	}
	if err := injectHeaders(page, params.Headers); err != nil {
		return nil, fmt.Errorf("inject headers failed: %w", err)
	}

	page = page.Context(ctx)

	if params.WaitSelector != "" {
		if err := page.Navigate(rawURL); err != nil {
			return nil, fmt.Errorf("navigate to %s failed: %w", rawURL, err)
		}
		waitTimeout := defaultPageTimeout
		if params.WaitTimeout > 0 {
			waitTimeout = params.WaitTimeout
		}
		waitPage := page.Timeout(waitTimeout)
		if _, err := waitPage.ElementX(params.WaitSelector); err != nil {
			return nil, fmt.Errorf("wait xpath %q failed: %w", params.WaitSelector, err)
		}
	} else {
		wait := page.WaitNavigation(proto.PageLifecycleEventNameNetworkAlmostIdle)
		if err := page.Navigate(rawURL); err != nil {
			return nil, fmt.Errorf("navigate to %s failed: %w", rawURL, err)
		}
		wait()
	}

	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("get page html failed: %w", err)
	}

	pageCookies := extractCookies(page)

	return &NavigateResult{
		HTML:    []byte(html),
		Cookies: pageCookies,
	}, nil
}

func injectCookies(page *rod.Page, rawURL string, cookies []*http.Cookie) error {
	if len(cookies) == 0 {
		return nil
	}
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url %q: %w", rawURL, err)
	}
	domain := parsed.Hostname()
	seen := make(map[string]struct{}, len(cookies))
	for _, c := range cookies {
		if _, dup := seen[c.Name]; dup {
			continue
		}
		seen[c.Name] = struct{}{}
		if _, setErr := (proto.NetworkSetCookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: domain,
			Path:   "/",
		}).Call(page); setErr != nil {
			return fmt.Errorf("set cookie %q: %w", c.Name, setErr)
		}
	}
	return nil
}

func injectHeaders(page *rod.Page, headers http.Header) error {
	if len(headers) == 0 {
		return nil
	}
	dict := make([]string, 0, len(headers)*2)
	for k, vs := range headers {
		if len(vs) > 0 {
			dict = append(dict, k, vs[0])
		}
	}
	_, err := page.SetExtraHeaders(dict)
	return err
}

func extractCookies(page *rod.Page) []*http.Cookie {
	rodCookies, err := page.Cookies(nil)
	if err != nil || len(rodCookies) == 0 {
		return nil
	}
	result := make([]*http.Cookie, 0, len(rodCookies))
	for _, c := range rodCookies {
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}
		if c.Expires > 0 {
			hc.Expires = time.Unix(int64(c.Expires), 0)
		}
		switch c.SameSite {
		case proto.NetworkCookieSameSiteLax:
			hc.SameSite = http.SameSiteLaxMode
		case proto.NetworkCookieSameSiteStrict:
			hc.SameSite = http.SameSiteStrictMode
		case proto.NetworkCookieSameSiteNone:
			hc.SameSite = http.SameSiteNoneMode
		}
		result = append(result, hc)
	}
	return result
}

func (rb *rodBrowser) Close() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.idleTimer != nil {
		rb.idleTimer.Stop()
		rb.idleTimer = nil
	}
	if rb.browser != nil {
		if rb.remote {
			rb.browser = nil
			return nil
		}
		err := rb.browser.Close()
		rb.browser = nil
		return err
	}
	return nil
}

func (rb *rodBrowser) startIdleTimerLocked() {
	if rb.idleTimer != nil {
		rb.idleTimer.Stop()
	}
	rb.idleTimer = time.AfterFunc(browserIdleTimeout, func() {
		rb.mu.Lock()
		defer rb.mu.Unlock()
		if rb.browser != nil && rb.activeCount == 0 {
			_ = rb.browser.Close()
			rb.browser = nil
		}
		rb.idleTimer = nil
	})
}
