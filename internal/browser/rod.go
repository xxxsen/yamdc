package browser

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
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
	mu        sync.Mutex
	browser   *rod.Browser
	remote    bool
	remoteURL string
	proxy     string
	dataDir   string
	idleTimer *time.Timer
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
		if !rb.remote {
			rb.resetIdleTimerLocked()
		}
		return rb.browser, nil
	}

	var controlURL string
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
	}

	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("connect browser failed: %w", err)
	}
	rb.browser = b
	if !rb.remote {
		rb.startIdleTimerLocked()
	}
	return rb.browser, nil
}

func (rb *rodBrowser) Navigate(ctx context.Context, rawURL string, params *Params) ([]byte, error) {
	b, err := rb.ensureBrowser()
	if err != nil {
		return nil, err
	}

	if !rb.remote {
		rb.pauseIdleTimer()
		defer rb.resumeIdleTimer()
	}

	page, err := stealth.Page(b)
	if err != nil {
		return nil, fmt.Errorf("create stealth page failed: %w", err)
	}
	defer func() { _ = page.Close() }()

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
	return []byte(html), nil
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

func (rb *rodBrowser) pauseIdleTimer() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.idleTimer != nil {
		rb.idleTimer.Stop()
	}
}

func (rb *rodBrowser) resumeIdleTimer() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.startIdleTimerLocked()
}

func (rb *rodBrowser) startIdleTimerLocked() {
	rb.idleTimer = time.AfterFunc(browserIdleTimeout, func() {
		rb.mu.Lock()
		defer rb.mu.Unlock()
		if rb.browser != nil {
			_ = rb.browser.Close()
			rb.browser = nil
		}
		rb.idleTimer = nil
	})
}

func (rb *rodBrowser) resetIdleTimerLocked() {
	if rb.idleTimer != nil {
		rb.idleTimer.Stop()
	}
	rb.startIdleTimerLocked()
}
