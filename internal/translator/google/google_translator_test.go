package google

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Default(t *testing.T) {
	tr := New()
	require.NotNil(t, tr)
	assert.Equal(t, "google", tr.Name())
}

func TestNew_WithProxyURL(t *testing.T) {
	tr := New(WithProxyURL("http://proxy:8080"))
	require.NotNil(t, tr)
	assert.Equal(t, "google", tr.Name())
}

func TestNew_WithServiceHosts(t *testing.T) {
	tr := New(WithServiceHosts("127.0.0.1:12345"))
	require.NotNil(t, tr)
	assert.Equal(t, "google", tr.Name())
}

func TestNew_WithAllOptions(t *testing.T) {
	tr := New(
		WithProxyURL("http://proxy:8080"),
		WithServiceHosts("host1:1234", "host2:5678"),
	)
	require.NotNil(t, tr)
}

func TestName(t *testing.T) {
	tr := New()
	assert.Equal(t, "google", tr.Name())
}

func TestWithProxyURL_SetsProxy(t *testing.T) {
	c := &config{}
	WithProxyURL("http://p:1234")(c)
	assert.Equal(t, "http://p:1234", c.proxy)
}

func TestWithServiceHosts_SetsHosts(t *testing.T) {
	c := &config{}
	WithServiceHosts("a:1", "b:2")(c)
	assert.Equal(t, []string{"a:1", "b:2"}, c.serviceURLs)
}

func TestWithServiceHosts_CopiesSlice(t *testing.T) {
	original := []string{"a:1", "b:2"}
	c := &config{}
	WithServiceHosts(original...)(c)
	original[0] = "modified"
	assert.Equal(t, "a:1", c.serviceURLs[0], "should not mutate via original slice")
}

func TestTranslate_SuccessPath(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/translate_a/single" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sentences":[{"trans":"你好","orig":"hello","backend":1}]}`))
			return
		}
		_, _ = w.Write([]byte(`tkk:'0.0'`))
	}))
	defer srv.Close()

	host := srv.Listener.Addr().String()
	tr := New(WithServiceHosts(host))
	result, err := tr.Translate(nil, "hello", "en", "zh") //nolint:staticcheck
	require.NoError(t, err)
	assert.Equal(t, "你好", result)
}

func TestTranslate_ErrorPath(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/translate_a/single" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`tkk:'0.0'`))
	}))
	defer srv.Close()

	host := srv.Listener.Addr().String()
	tr := New(WithServiceHosts(host))
	_, err := tr.Translate(nil, "hello", "en", "zh") //nolint:staticcheck
	require.Error(t, err)
	assert.Contains(t, err.Error(), "google translate failed")
}

func TestNew_EmptyServiceURLs(t *testing.T) {
	tr := New(WithServiceHosts())
	require.NotNil(t, tr)
	inner := tr.(*googleTranslator)
	assert.NotNil(t, inner.t)
}
