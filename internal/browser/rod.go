package browser

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/xxxsen/common/logutil"
)

const (
	defaultPageTimeout = 60 * time.Second
	browserIdleTimeout = 30 * time.Second
)

type rodBrowser struct {
	provider browserProvider
}

func NewNavigator(cfg *Config) INavigator {
	if cfg.RemoteURL != "" {
		return NewRemoteNavigator(cfg.RemoteURL)
	}
	return NewRodNavigator(cfg.DataDir, cfg.Proxy)
}

func NewRodNavigator(dataDir, proxy string) INavigator {
	return &rodBrowser{provider: newLocalProvider(dataDir, proxy)}
}

// NewRemoteNavigator connects to an existing browser via CDP remote
// debugging. The browser is NOT managed by us — no idle-close, no
// process kill on Close().
// remoteURL accepts formats like "9222", ":9222", "host:9222",
// "ws://host:9222", "http://host:9222".
func NewRemoteNavigator(remoteURL string) INavigator {
	return &rodBrowser{provider: newRemoteProvider(remoteURL)}
}

func (rb *rodBrowser) Navigate(ctx context.Context, rawURL string, params *Params) (*NavigateResult, error) {
	b, err := rb.provider.Acquire()
	if err != nil {
		return nil, fmt.Errorf("acquire browser failed: %w", err)
	}
	defer func() {
		rb.provider.Release()
		logutil.GetLogger(ctx).Debug("browser navigate released")
	}()

	page, err := prepareStealthPage(b, rawURL, params)
	if err != nil {
		return nil, err
	}
	defer func() { _ = page.Close() }()

	page = page.Context(ctx)
	if err := navigatePage(page, rawURL, params); err != nil {
		return nil, err
	}

	return collectNavigateResult(page)
}

func prepareStealthPage(b *rod.Browser, rawURL string, params *Params) (*rod.Page, error) {
	page, err := stealth.Page(b)
	if err != nil {
		return nil, fmt.Errorf("create stealth page failed: %w", err)
	}
	if err := injectCookies(page, rawURL, params.Cookies); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("inject cookies failed: %w", err)
	}
	if err := injectHeaders(page, params.Headers); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("inject headers failed: %w", err)
	}
	return page, nil
}

func collectNavigateResult(page *rod.Page) (*NavigateResult, error) {
	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("get page html failed: %w", err)
	}
	return &NavigateResult{
		HTML:    []byte(html),
		Cookies: extractCookies(page),
	}, nil
}

func (rb *rodBrowser) Close() error {
	if err := rb.provider.Close(); err != nil {
		return fmt.Errorf("close browser provider failed: %w", err)
	}
	return nil
}

func navigatePage(page *rod.Page, rawURL string, params *Params) error {
	if params.WaitSelector != "" {
		return navigateWithSelector(page, rawURL, params)
	}
	return navigateWithIdle(page, rawURL)
}

func navigateWithSelector(page *rod.Page, rawURL string, params *Params) error {
	if err := page.Navigate(rawURL); err != nil {
		return fmt.Errorf("navigate to %s failed: %w", rawURL, err)
	}
	waitTimeout := defaultPageTimeout
	if params.WaitTimeout > 0 {
		waitTimeout = params.WaitTimeout
	}
	waitPage := page.Timeout(waitTimeout)
	if _, err := waitPage.ElementX(params.WaitSelector); err != nil {
		return fmt.Errorf("wait xpath %q failed: %w", params.WaitSelector, err)
	}
	return nil
}

func navigateWithIdle(page *rod.Page, rawURL string) error {
	wait := page.WaitNavigation(proto.PageLifecycleEventNameNetworkAlmostIdle)
	if err := page.Navigate(rawURL); err != nil {
		return fmt.Errorf("navigate to %s failed: %w", rawURL, err)
	}
	wait()
	return nil
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
	if err != nil {
		return fmt.Errorf("set extra headers failed: %w", err)
	}
	return nil
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
