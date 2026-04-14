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
)

const (
	defaultPageTimeout = 60 * time.Second
	browserIdleTimeout = 30 * time.Second
)

type rodBrowser struct {
	mu        sync.Mutex
	browser   *rod.Browser
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

func (rb *rodBrowser) ensureBrowser() (*rod.Browser, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.browser != nil {
		rb.resetIdleTimerLocked()
		return rb.browser, nil
	}
	launcher.DefaultBrowserDir = filepath.Join(rb.dataDir, "browser")
	l := launcher.New().
		NoSandbox(true).
		Set("disable-gpu").
		Set("disable-dev-shm-usage")
	if rb.proxy != "" {
		l = l.Proxy(rb.proxy)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch browser failed: %w", err)
	}
	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("connect browser failed: %w", err)
	}
	rb.browser = b
	rb.startIdleTimerLocked()
	return rb.browser, nil
}

func (rb *rodBrowser) Navigate(ctx context.Context, rawURL string, params *Params) ([]byte, error) {
	b, err := rb.ensureBrowser()
	if err != nil {
		return nil, err
	}

	rb.pauseIdleTimer()
	defer rb.resumeIdleTimer()

	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("create page failed: %w", err)
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
