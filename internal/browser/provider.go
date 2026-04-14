package browser

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// browserProvider abstracts how a *rod.Browser instance is obtained and
// released. Acquire and Release must be called in pairs.
type browserProvider interface {
	Acquire() (*rod.Browser, error)
	Release()
	Close() error
}

// ---------------------------------------------------------------------------
// localProvider — manages a local headless Chromium process
// ---------------------------------------------------------------------------

type localProvider struct {
	mu          sync.Mutex
	browser     *rod.Browser
	dataDir     string
	proxy       string
	activeCount int
	idleTimer   *time.Timer
}

func newLocalProvider(dataDir, proxy string) browserProvider {
	return &localProvider{dataDir: dataDir, proxy: proxy}
}

func (p *localProvider) Acquire() (*rod.Browser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureBrowserLocked(); err != nil {
		return nil, err
	}
	p.activeCount++
	if p.idleTimer != nil {
		p.idleTimer.Stop()
		p.idleTimer = nil
	}
	return p.browser, nil
}

func (p *localProvider) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeCount--
	if p.activeCount == 0 && p.browser != nil {
		p.startIdleTimerLocked()
	}
}

func (p *localProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.idleTimer != nil {
		p.idleTimer.Stop()
		p.idleTimer = nil
	}
	if p.browser != nil {
		err := p.browser.Close()
		p.browser = nil
		return err
	}
	return nil
}

func (p *localProvider) ensureBrowserLocked() error {
	if p.browser != nil {
		return nil
	}
	launcher.DefaultBrowserDir = filepath.Join(p.dataDir, "browser")
	l := launcher.New().
		Set("headless", "new").
		NoSandbox(true).
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Set("window-size", "1920,1080")
	if p.proxy != "" {
		l = l.Proxy(p.proxy)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser failed: %w", err)
	}
	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		l.Kill()
		return fmt.Errorf("connect browser failed: %w", err)
	}
	p.browser = b
	return nil
}

func (p *localProvider) startIdleTimerLocked() {
	if p.idleTimer != nil {
		p.idleTimer.Stop()
	}
	p.idleTimer = time.AfterFunc(browserIdleTimeout, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.browser != nil && p.activeCount == 0 {
			_ = p.browser.Close()
			p.browser = nil
		}
		p.idleTimer = nil
	})
}

// ---------------------------------------------------------------------------
// remoteProvider — connects to an external browser via CDP
// ---------------------------------------------------------------------------

type remoteProvider struct {
	mu        sync.Mutex
	browser   *rod.Browser
	remoteURL string
}

func newRemoteProvider(remoteURL string) browserProvider {
	return &remoteProvider{remoteURL: remoteURL}
}

func (p *remoteProvider) Acquire() (*rod.Browser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.browser != nil {
		return p.browser, nil
	}
	wsURL, err := launcher.ResolveURL(p.remoteURL)
	if err != nil {
		return nil, fmt.Errorf("resolve remote browser URL %q failed: %w", p.remoteURL, err)
	}
	b := rod.New().ControlURL(wsURL)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("connect remote browser failed: %w", err)
	}
	p.browser = b
	return p.browser, nil
}

func (p *remoteProvider) Release() {}

func (p *remoteProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.browser != nil {
		p.browser = nil
	}
	return nil
}
