package client

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- 正常 case: MustCookieJar 返回一个可用的 jar ---

func TestMustCookieJarReturnsNonNil(t *testing.T) {
	jar := MustCookieJar()
	require.NotNil(t, jar)
	assert.IsType(t, &cookiejar.Jar{}, jar)
}

// --- 正常 case: 返回的 jar 功能完整, 可以存取 cookie ---

func TestMustCookieJarSetAndGetCookies(t *testing.T) {
	jar := MustCookieJar()
	u, err := url.Parse("https://example.com/")
	require.NoError(t, err)

	jar.SetCookies(u, []*http.Cookie{{Name: "sid", Value: "abc", Path: "/"}})
	got := jar.Cookies(u)
	require.Len(t, got, 1)
	assert.Equal(t, "sid", got[0].Name)
	assert.Equal(t, "abc", got[0].Value)
}

// --- 边缘 case: 每次调用返回独立实例, 不共享状态 ---

func TestMustCookieJarReturnsFreshInstance(t *testing.T) {
	a := MustCookieJar()
	b := MustCookieJar()
	assert.NotSame(t, a, b)

	u, err := url.Parse("https://example.com/")
	require.NoError(t, err)
	a.SetCookies(u, []*http.Cookie{{Name: "sid", Value: "from-a", Path: "/"}})
	assert.Empty(t, b.Cookies(u), "jar b should not see cookies set on jar a")
}
