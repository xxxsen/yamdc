package browser

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- localProvider state-based tests ----------

func TestLocalProvider_AcquireRelease_WithoutBrowser(t *testing.T) {
	p := newLocalProvider(t.TempDir(), "").(*localProvider)
	p.mu.Lock()
	assert.Nil(t, p.browser)
	assert.Equal(t, 0, p.activeCount)
	p.mu.Unlock()

	p.Release()
	p.mu.Lock()
	assert.Equal(t, -1, p.activeCount)
	p.mu.Unlock()

	err := p.Close()
	assert.NoError(t, err)
}

func TestLocalProvider_Close_NilBrowser(t *testing.T) {
	p := newLocalProvider(t.TempDir(), "").(*localProvider)
	err := p.Close()
	assert.NoError(t, err)
}

func TestLocalProvider_Close_StopsIdleTimer(t *testing.T) {
	p := newLocalProvider(t.TempDir(), "").(*localProvider)
	p.mu.Lock()
	p.idleTimer = time.AfterFunc(1*time.Hour, func() {})
	p.mu.Unlock()

	err := p.Close()
	assert.NoError(t, err)
	p.mu.Lock()
	assert.Nil(t, p.idleTimer)
	p.mu.Unlock()
}

func TestLocalProvider_StartIdleTimer_Replaces(t *testing.T) {
	p := newLocalProvider(t.TempDir(), "").(*localProvider)
	p.mu.Lock()
	p.startIdleTimerLocked()
	first := p.idleTimer
	p.startIdleTimerLocked()
	second := p.idleTimer
	p.mu.Unlock()
	assert.NotNil(t, first)
	assert.NotNil(t, second)
	first.Stop()
	second.Stop()
}

func TestNewLocalProvider_WithProxy(t *testing.T) {
	p := newLocalProvider(t.TempDir(), "http://proxy:8080").(*localProvider)
	assert.Equal(t, "http://proxy:8080", p.proxy)
	_ = p.Close()
}

func TestLocalProvider_Acquire_BrowserAlreadySet(t *testing.T) {
	tests := []struct {
		name          string
		hasIdleTimer  bool
		wantTimerNil  bool
		wantActive    int
	}{
		{"no idle timer", false, true, 1},
		{"with idle timer", true, true, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newLocalProvider(t.TempDir(), "").(*localProvider)
			p.browser = rod.New()
			if tc.hasIdleTimer {
				p.idleTimer = time.AfterFunc(time.Hour, func() {})
			}

			got, err := p.Acquire()
			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tc.wantActive, p.activeCount)
			if tc.wantTimerNil {
				assert.Nil(t, p.idleTimer)
			}
		})
	}
}

func TestLocalProvider_Release_StartsIdleTimer(t *testing.T) {
	p := newLocalProvider(t.TempDir(), "").(*localProvider)
	p.browser = rod.New()
	p.activeCount = 1

	p.Release()
	p.mu.Lock()
	assert.Equal(t, 0, p.activeCount)
	assert.NotNil(t, p.idleTimer, "idle timer should start when activeCount reaches 0")
	p.idleTimer.Stop()
	p.mu.Unlock()
}

func TestLocalProvider_Release_NoTimerWithoutBrowser(t *testing.T) {
	p := newLocalProvider(t.TempDir(), "").(*localProvider)
	p.activeCount = 1

	p.Release()
	p.mu.Lock()
	assert.Equal(t, 0, p.activeCount)
	assert.Nil(t, p.idleTimer, "idle timer should not start without browser")
	p.mu.Unlock()
}

func TestLocalProvider_EnsureBrowserLocked_AlreadySet(t *testing.T) {
	p := newLocalProvider(t.TempDir(), "").(*localProvider)
	p.browser = rod.New()

	p.mu.Lock()
	err := p.ensureBrowserLocked()
	p.mu.Unlock()

	assert.NoError(t, err)
}

// ---------- localProvider full lifecycle (real browser) ----------

func TestLocalProvider_FullLifecycle(t *testing.T) {
	dataDir := filepath.Join(os.TempDir(), "yamdc-test-browser-lifecycle")
	p := newLocalProvider(dataDir, "").(*localProvider)
	t.Cleanup(func() { _ = p.Close() })

	b, err := p.Acquire()
	if err != nil {
		t.Skipf("cannot launch browser: %v", err)
	}
	require.NotNil(t, b)
	assert.Equal(t, 1, p.activeCount)

	b2, err := p.Acquire()
	require.NoError(t, err)
	assert.Equal(t, b, b2, "should return same browser instance")
	assert.Equal(t, 2, p.activeCount)

	p.Release()
	p.mu.Lock()
	assert.Equal(t, 1, p.activeCount)
	assert.Nil(t, p.idleTimer)
	p.mu.Unlock()

	p.Release()
	p.mu.Lock()
	assert.Equal(t, 0, p.activeCount)
	assert.NotNil(t, p.idleTimer)
	p.idleTimer.Stop()
	p.mu.Unlock()

	err = p.Close()
	assert.NoError(t, err)
	assert.Nil(t, p.browser)
}

func TestLocalProvider_FullLifecycle_WithProxy(t *testing.T) {
	dataDir := filepath.Join(os.TempDir(), "yamdc-test-browser-proxy")
	p := newLocalProvider(dataDir, "http://nonexistent-proxy:9999").(*localProvider)
	t.Cleanup(func() { _ = p.Close() })

	b, err := p.Acquire()
	if err != nil {
		t.Skipf("cannot launch browser with proxy: %v", err)
	}
	require.NotNil(t, b)
	_ = p.Close()
}

// ---------- remoteProvider tests ----------

func TestRemoteProvider_AcquireConnectFailure(t *testing.T) {
	p := newRemoteProvider("http://127.0.0.1:1").(*remoteProvider)
	_, err := p.Acquire()
	assert.Error(t, err)
}

func TestRemoteProvider_Acquire_CachedBrowser(t *testing.T) {
	p := newRemoteProvider("ws://localhost:0").(*remoteProvider)
	b := rod.New()
	p.browser = b

	got, err := p.Acquire()
	require.NoError(t, err)
	assert.Equal(t, b, got)
}

func TestRemoteProvider_Close(t *testing.T) {
	p := newRemoteProvider("ws://localhost:0").(*remoteProvider)
	err := p.Close()
	assert.NoError(t, err)
}

func TestRemoteProvider_Close_WithBrowser(t *testing.T) {
	p := newRemoteProvider("ws://localhost:0").(*remoteProvider)
	p.browser = rod.New()

	err := p.Close()
	assert.NoError(t, err)
	assert.Nil(t, p.browser)
}

func TestRemoteProvider_Release(t *testing.T) {
	p := newRemoteProvider("ws://localhost:0").(*remoteProvider)
	p.Release()
}

// ---------- idle timer callback ----------

func TestLocalProvider_IdleTimerCallback(t *testing.T) {
	saved := browserIdleTimeout
	browserIdleTimeout = 10 * time.Millisecond
	t.Cleanup(func() { browserIdleTimeout = saved })

	dataDir := filepath.Join(os.TempDir(), "yamdc-test-idle-callback")
	p := newLocalProvider(dataDir, "").(*localProvider)

	b, err := p.Acquire()
	if err != nil {
		t.Skipf("cannot launch browser: %v", err)
	}
	require.NotNil(t, b)

	p.Release()

	time.Sleep(200 * time.Millisecond)

	p.mu.Lock()
	assert.Nil(t, p.browser, "browser should be closed by idle timer")
	assert.Nil(t, p.idleTimer, "timer reference should be cleared")
	p.mu.Unlock()
}

func TestLocalProvider_IdleTimerCallback_ActiveCount(t *testing.T) {
	saved := browserIdleTimeout
	browserIdleTimeout = 10 * time.Millisecond
	t.Cleanup(func() { browserIdleTimeout = saved })

	p := newLocalProvider(t.TempDir(), "").(*localProvider)
	p.browser = rod.New()
	p.activeCount = 1

	p.mu.Lock()
	p.startIdleTimerLocked()
	p.mu.Unlock()

	time.Sleep(200 * time.Millisecond)

	p.mu.Lock()
	assert.NotNil(t, p.browser, "browser should NOT be closed when activeCount > 0")
	if p.idleTimer != nil {
		p.idleTimer.Stop()
	}
	p.mu.Unlock()
}
