package browser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- shared test browser ----------

var (
	testBrowserOnce       sync.Once
	testBrowser           *rod.Browser
	testBrowserControlURL string
	testBrowserSkip       string
)

func getTestBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	testBrowserOnce.Do(func() {
		browserDir := filepath.Join(os.TempDir(), "yamdc-test-browser-shared", "browser")
		launcher.DefaultBrowserDir = browserDir
		l := launcher.New().
			NoSandbox(true).
			Set("disable-gpu").
			Set("disable-dev-shm-usage")
		u, err := l.Launch()
		if err != nil {
			testBrowserSkip = fmt.Sprintf("cannot launch browser: %v", err)
			return
		}
		testBrowserControlURL = u
		b := rod.New().ControlURL(u)
		if err := b.Connect(); err != nil {
			l.Kill()
			testBrowserSkip = fmt.Sprintf("cannot connect: %v", err)
			return
		}
		testBrowser = b
	})
	if testBrowserSkip != "" {
		t.Skip(testBrowserSkip)
	}
	return testBrowser
}

func TestMain(m *testing.M) {
	code := m.Run()
	if testBrowser != nil {
		_ = testBrowser.Close()
	}
	os.Exit(code)
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body><div id="content">hello</div></body></html>`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---------- mock browserProvider ----------

type mockBrowserProvider struct {
	acquireFunc func() (*rod.Browser, error)
	releaseFunc func()
	closeFunc   func() error
}

func (m *mockBrowserProvider) Acquire() (*rod.Browser, error) {
	if m.acquireFunc != nil {
		return m.acquireFunc()
	}
	return nil, errors.New("no acquire func")
}

func (m *mockBrowserProvider) Release() {
	if m.releaseFunc != nil {
		m.releaseFunc()
	}
}

func (m *mockBrowserProvider) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// ---------- NewNavigator factory tests ----------

func TestNewNavigator_RemoteURL(t *testing.T) {
	nav := NewNavigator(&Config{RemoteURL: "ws://host:9222"})
	assert.NotNil(t, nav)
	rb, ok := nav.(*rodBrowser)
	require.True(t, ok)
	_, isRemote := rb.provider.(*remoteProvider)
	assert.True(t, isRemote)
}

func TestNewNavigator_Local(t *testing.T) {
	nav := NewNavigator(&Config{DataDir: "/tmp", Proxy: ""})
	assert.NotNil(t, nav)
	rb, ok := nav.(*rodBrowser)
	require.True(t, ok)
	_, isLocal := rb.provider.(*localProvider)
	assert.True(t, isLocal)
}

func TestNewRodNavigator(t *testing.T) {
	nav := NewRodNavigator("/tmp/test", "http://proxy:8080")
	assert.NotNil(t, nav)
	_ = nav.Close()
}

func TestNewRemoteNavigator(t *testing.T) {
	nav := NewRemoteNavigator("ws://host:9222")
	assert.NotNil(t, nav)
	_ = nav.Close()
}

// ---------- rodBrowser.Close tests ----------

func TestRodBrowser_Close_Success(t *testing.T) {
	nav := NewRemoteNavigator("ws://localhost:0")
	err := nav.Close()
	assert.NoError(t, err)
}

func TestRodBrowser_Close_ProviderAlreadyClosed(t *testing.T) {
	nav := NewRemoteNavigator("ws://localhost:0")
	require.NoError(t, nav.Close())
	require.NoError(t, nav.Close())
}

func TestRodBrowser_Close_ProviderError(t *testing.T) {
	rb := &rodBrowser{
		provider: &mockBrowserProvider{
			closeFunc: func() error {
				return errors.New("close error")
			},
		},
	}
	err := rb.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close browser provider failed")
}

func TestRodBrowser_Close_ProviderSuccess(t *testing.T) {
	rb := &rodBrowser{
		provider: &mockBrowserProvider{
			closeFunc: func() error {
				return nil
			},
		},
	}
	err := rb.Close()
	require.NoError(t, err)
}

// ---------- rodBrowser.Navigate error paths ----------

func TestRodBrowser_Navigate_AcquireError(t *testing.T) {
	nav := NewRemoteNavigator("http://127.0.0.1:1")
	_, err := nav.Navigate(context.Background(), "http://example.com", &Params{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire browser failed")
	_ = nav.Close()
}

func TestRodBrowser_Navigate_AcquireError_MockProvider(t *testing.T) {
	rb := &rodBrowser{
		provider: &mockBrowserProvider{
			acquireFunc: func() (*rod.Browser, error) {
				return nil, errors.New("browser unavailable")
			},
		},
	}
	_, err := rb.Navigate(context.Background(), "http://example.com", &Params{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire browser failed")
}

func TestRodBrowser_Navigate_InjectCookiesError(t *testing.T) {
	b := getTestBrowser(t)

	released := false
	rb := &rodBrowser{
		provider: &mockBrowserProvider{
			acquireFunc: func() (*rod.Browser, error) { return b, nil },
			releaseFunc: func() { released = true },
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := rb.Navigate(ctx, "http://[::1", &Params{
		Cookies: []*http.Cookie{{Name: "a", Value: "1"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inject cookies failed")
	assert.True(t, released)
}

// ---------- rodBrowser.Navigate success paths ----------

func TestRodBrowser_Navigate_WithSelector(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	released := false
	rb := &rodBrowser{
		provider: &mockBrowserProvider{
			acquireFunc: func() (*rod.Browser, error) { return b, nil },
			releaseFunc: func() { released = true },
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := rb.Navigate(ctx, srv.URL, &Params{
		WaitSelector: `//*[@id="content"]`,
		WaitTimeout:  10 * time.Second,
	})
	require.NoError(t, err)
	assert.Contains(t, string(result.HTML), "hello")
	assert.True(t, released)
}

func TestRodBrowser_Navigate_WithIdle(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	rb := &rodBrowser{
		provider: &mockBrowserProvider{
			acquireFunc: func() (*rod.Browser, error) { return b, nil },
			releaseFunc: func() {},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := rb.Navigate(ctx, srv.URL, &Params{})
	require.NoError(t, err)
	assert.Contains(t, string(result.HTML), "hello")
}

// ---------- navigatePage routing ----------

func TestNavigatePage(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	tests := []struct {
		name   string
		params *Params
	}{
		{"routes to navigateWithSelector", &Params{WaitSelector: `//*[@id="content"]`, WaitTimeout: 5 * time.Second}},
		{"routes to navigateWithIdle", &Params{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := stealth.Page(b)
			require.NoError(t, err)
			defer func() { _ = page.Close() }()

			err = navigatePage(page, srv.URL, tc.params)
			assert.NoError(t, err)
		})
	}
}

// ---------- navigateWithSelector ----------

func TestNavigateWithSelector(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	tests := []struct {
		name    string
		params  *Params
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success with explicit timeout",
			params: &Params{WaitSelector: `//*[@id="content"]`, WaitTimeout: 5 * time.Second},
		},
		{
			name:   "success with default timeout",
			params: &Params{WaitSelector: `//*[@id="content"]`},
		},
		{
			name:    "element not found",
			params:  &Params{WaitSelector: `//*[@id="nonexistent"]`, WaitTimeout: 1 * time.Second},
			wantErr: true,
			errMsg:  "wait xpath",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := stealth.Page(b)
			require.NoError(t, err)
			defer func() { _ = page.Close() }()

			err = navigateWithSelector(page, srv.URL, tc.params)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------- navigateWithIdle ----------

func TestNavigateWithIdle(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	page, err := stealth.Page(b)
	require.NoError(t, err)
	defer func() { _ = page.Close() }()

	err = navigateWithIdle(page, srv.URL)
	assert.NoError(t, err)
}

// ---------- prepareStealthPage ----------

func TestPrepareStealthPage(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	tests := []struct {
		name   string
		params *Params
	}{
		{
			name:   "basic no cookies no headers",
			params: &Params{},
		},
		{
			name: "with cookies and headers",
			params: &Params{
				Cookies: []*http.Cookie{{Name: "ck1", Value: "v1"}, {Name: "ck2", Value: "v2"}},
				Headers: http.Header{"X-Custom": {"val"}},
			},
		},
		{
			name:   "nil cookies and nil headers",
			params: &Params{Cookies: nil, Headers: nil},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := prepareStealthPage(b, srv.URL, tc.params)
			require.NoError(t, err)
			require.NotNil(t, page)
			_ = page.Close()
		})
	}
}

// ---------- collectNavigateResult ----------

func TestCollectNavigateResult(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	page, err := stealth.Page(b)
	require.NoError(t, err)
	defer func() { _ = page.Close() }()

	err = page.Navigate(srv.URL)
	require.NoError(t, err)
	page.MustWaitStable()

	result, err := collectNavigateResult(page)
	require.NoError(t, err)
	assert.Contains(t, string(result.HTML), "hello")
}

// ---------- injectCookies (rod.go) ----------

func TestInjectCookiesRod(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	tests := []struct {
		name    string
		rawURL  string
		cookies []*http.Cookie
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil cookies",
			rawURL:  srv.URL,
			cookies: nil,
		},
		{
			name:    "empty cookies",
			rawURL:  srv.URL,
			cookies: []*http.Cookie{},
		},
		{
			name:    "single cookie",
			rawURL:  srv.URL,
			cookies: []*http.Cookie{{Name: "a", Value: "1"}},
		},
		{
			name:    "multiple cookies",
			rawURL:  srv.URL,
			cookies: []*http.Cookie{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}},
		},
		{
			name:    "duplicate cookie names",
			rawURL:  srv.URL,
			cookies: []*http.Cookie{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}, {Name: "a", Value: "3"}},
		},
		{
			name:    "invalid url",
			rawURL:  "http://[::1",
			cookies: []*http.Cookie{{Name: "a", Value: "1"}},
			wantErr: true,
			errMsg:  "parse url",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.cookies) == 0 {
				err := injectCookies(nil, tc.rawURL, tc.cookies)
				assert.NoError(t, err)
				return
			}
			if tc.wantErr {
				err := injectCookies(nil, tc.rawURL, tc.cookies)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				return
			}
			page, err := stealth.Page(b)
			require.NoError(t, err)
			defer func() { _ = page.Close() }()

			err = injectCookies(page, tc.rawURL, tc.cookies)
			assert.NoError(t, err)
		})
	}
}

// ---------- injectHeaders (rod.go) ----------

func TestInjectHeadersRod(t *testing.T) {
	b := getTestBrowser(t)

	tests := []struct {
		name    string
		headers http.Header
		needsPage bool
	}{
		{"nil headers", nil, false},
		{"empty headers", http.Header{}, false},
		{"single header", http.Header{"X-Custom": {"val"}}, true},
		{"multiple headers", http.Header{"X-A": {"1"}, "X-B": {"2"}}, true},
		{"header with empty values", http.Header{"X-Empty": {}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.needsPage {
				err := injectHeaders(nil, tc.headers)
				assert.NoError(t, err)
				return
			}
			page, err := stealth.Page(b)
			require.NoError(t, err)
			defer func() { _ = page.Close() }()

			err = injectHeaders(page, tc.headers)
			assert.NoError(t, err)
		})
	}
}

// ---------- extractCookies (rod.go) ----------

func TestExtractCookiesRod(t *testing.T) {
	b := getTestBrowser(t)
	srv := newTestServer(t)

	t.Run("no cookies", func(t *testing.T) {
		page, err := stealth.Page(b)
		require.NoError(t, err)
		defer func() { _ = page.Close() }()

		result := extractCookies(page)
		assert.Empty(t, result)
	})

	t.Run("with cookies", func(t *testing.T) {
		page, err := stealth.Page(b)
		require.NoError(t, err)
		defer func() { _ = page.Close() }()

		err = page.Navigate(srv.URL)
		require.NoError(t, err)
		page.MustWaitStable()

		err = injectCookies(page, srv.URL, []*http.Cookie{
			{Name: "test_ck", Value: "val"},
		})
		require.NoError(t, err)

		result := extractCookies(page)
		require.NotEmpty(t, result)
		found := false
		for _, c := range result {
			if c.Name == "test_ck" {
				found = true
				assert.Equal(t, "val", c.Value)
			}
		}
		assert.True(t, found, "injected cookie should be extracted")
	})

	t.Run("samesite and expires", func(t *testing.T) {
		page, err := stealth.Page(b)
		require.NoError(t, err)
		defer func() { _ = page.Close() }()

		err = page.Navigate(srv.URL)
		require.NoError(t, err)
		page.MustWaitStable()

		expireTime := time.Now().Add(time.Hour)
		setCookies := []struct {
			name     string
			sameSite proto.NetworkCookieSameSite
			secure   bool
			expires  proto.TimeSinceEpoch
		}{
			{"lax_ck", proto.NetworkCookieSameSiteLax, false, 0},
			{"strict_ck", proto.NetworkCookieSameSiteStrict, false, 0},
			{"none_ck", proto.NetworkCookieSameSiteNone, true, 0},
			{"exp_ck", "", false, proto.TimeSinceEpoch(expireTime.Unix())},
		}
		for _, sc := range setCookies {
			_, err := (proto.NetworkSetCookie{
				Name:     sc.name,
				Value:    "v",
				Domain:   "127.0.0.1",
				Path:     "/",
				SameSite: sc.sameSite,
				Secure:   sc.secure,
				Expires:  sc.expires,
			}).Call(page)
			require.NoError(t, err, "set cookie %s", sc.name)
		}

		result := extractCookies(page)
		require.NotEmpty(t, result)

		cookieMap := make(map[string]*http.Cookie)
		for _, c := range result {
			cookieMap[c.Name] = c
		}

		if c, ok := cookieMap["lax_ck"]; ok {
			assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
		}
		if c, ok := cookieMap["strict_ck"]; ok {
			assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
		}
		if c, ok := cookieMap["none_ck"]; ok {
			assert.Equal(t, http.SameSiteNoneMode, c.SameSite)
		}
		if c, ok := cookieMap["exp_ck"]; ok {
			assert.False(t, c.Expires.IsZero(), "cookie should have an expiry")
		}
	})
}

// ---------- error paths with closed page ----------

func TestNavigateWithSelector_ClosedPage(t *testing.T) {
	b := getTestBrowser(t)
	page, err := stealth.Page(b)
	require.NoError(t, err)
	_ = page.Close()

	err = navigateWithSelector(page, "http://example.com", &Params{
		WaitSelector: "//div",
		WaitTimeout:  time.Second,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "navigate to")
}

func TestNavigateWithIdle_ClosedPage(t *testing.T) {
	b := getTestBrowser(t)
	page, err := stealth.Page(b)
	require.NoError(t, err)
	_ = page.Close()

	err = navigateWithIdle(page, "http://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "navigate to")
}

func TestCollectNavigateResult_Error(t *testing.T) {
	b := getTestBrowser(t)
	page, err := stealth.Page(b)
	require.NoError(t, err)
	_ = page.Close()

	_, err = collectNavigateResult(page)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get page html failed")
}

func TestInjectCookiesRod_ClosedPage(t *testing.T) {
	b := getTestBrowser(t)
	page, err := stealth.Page(b)
	require.NoError(t, err)
	_ = page.Close()

	err = injectCookies(page, "http://example.com", []*http.Cookie{{Name: "a", Value: "1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set cookie")
}

func TestInjectHeadersRod_ClosedPage(t *testing.T) {
	b := getTestBrowser(t)
	page, err := stealth.Page(b)
	require.NoError(t, err)
	_ = page.Close()

	err = injectHeaders(page, http.Header{"X-Custom": {"val"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set extra headers failed")
}

// ---------- remoteProvider success path ----------

func TestRemoteProvider_Acquire_Success(t *testing.T) {
	_ = getTestBrowser(t)

	parsed, err := neturl.Parse(testBrowserControlURL)
	require.NoError(t, err)
	debugURL := "http://" + parsed.Host

	p := newRemoteProvider(debugURL).(*remoteProvider)
	t.Cleanup(func() { _ = p.Close() })

	b, err := p.Acquire()
	require.NoError(t, err)
	require.NotNil(t, b)

	b2, err := p.Acquire()
	require.NoError(t, err)
	assert.Equal(t, b, b2, "should return cached browser")
}

// ---------- prepareStealthPage with dead browser ----------

func TestPrepareStealthPage_BrowserClosed(t *testing.T) {
	browserDir := filepath.Join(os.TempDir(), "yamdc-test-stealth-closed", "browser")
	launcher.DefaultBrowserDir = browserDir
	l := launcher.New().NoSandbox(true).Set("disable-gpu").Set("disable-dev-shm-usage")
	u, err := l.Launch()
	if err != nil {
		t.Skipf("cannot launch browser: %v", err)
	}
	b := rod.New().ControlURL(u)
	require.NoError(t, b.Connect())
	_ = b.Close()

	_, err = prepareStealthPage(b, "http://example.com", &Params{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create stealth page failed")
}
