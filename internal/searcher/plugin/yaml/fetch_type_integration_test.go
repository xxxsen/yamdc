package yaml

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/browser"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
	"gopkg.in/yaml.v3"
)

// Environment variables:
//
//	FETCH_TYPE_TEST_URL      remote YAML plugin URL (required, skip if empty)
//	FETCH_TYPE_TEST_NUMBER   movie number to search (required, skip if empty)
//	FETCH_TYPE_TEST_WAIT     browser wait_selector xpath (optional)
func fetchTestEnv(t *testing.T) (string, string, string) {
	t.Helper()
	yamlURL := os.Getenv("FETCH_TYPE_TEST_URL")
	numberID := os.Getenv("FETCH_TYPE_TEST_NUMBER")
	waitSelector := os.Getenv("FETCH_TYPE_TEST_WAIT")
	if yamlURL == "" || numberID == "" {
		t.Skip("set FETCH_TYPE_TEST_URL and FETCH_TYPE_TEST_NUMBER to run")
	}
	return yamlURL, numberID, waitSelector
}

func fetchRemoteYAML(t *testing.T, url string) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	require.NoError(t, err, "create request")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "fetch remote YAML")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode, "remote YAML HTTP status")
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return data
}

func toBrowserYAML(t *testing.T, goHTTPData []byte, waitSelector string) []byte {
	t.Helper()
	var raw map[string]interface{}
	require.NoError(t, yaml.Unmarshal(goHTTPData, &raw))

	raw["fetch_type"] = "browser"
	raw["name"] = fmt.Sprintf("%s-browser", raw["name"])

	if waitSelector != "" {
		req, _ := raw["request"].(map[string]interface{})
		if req != nil {
			req["browser"] = map[string]interface{}{
				"wait_selector": waitSelector,
				"wait_timeout":  30,
			}
		}
	}

	out, err := yaml.Marshal(raw)
	require.NoError(t, err)
	return out
}

func doSearch(t *testing.T, yamlData []byte, httpCli client.IHTTPClient, numberID string) {
	t.Helper()
	plg := mustPluginFromYAML(t, string(yamlData))
	s, err := searcher.NewDefaultSearcher(plg.spec.name, plg,
		searcher.WithHTTPClient(httpCli),
		searcher.WithStorage(store.NewMemStorage()),
		searcher.WithSearchCache(false),
	)
	require.NoError(t, err)
	num, err := number.Parse(numberID)
	require.NoError(t, err)
	meta, ok, err := s.Search(context.Background(), num)
	require.NoError(t, err, "search should not fail")
	require.True(t, ok, "search should find result")
	require.NotNil(t, meta)

	t.Logf("Number:        %s", meta.Number)
	t.Logf("Title:         %s", meta.Title)
	t.Logf("Actors:        %v", meta.Actors)
	t.Logf("Studio:        %s", meta.Studio)
	t.Logf("Label:         %s", meta.Label)
	t.Logf("Series:        %s", meta.Series)
	t.Logf("Genres:        %v", meta.Genres)
	t.Logf("Duration:      %d", meta.Duration)
	t.Logf("ReleaseDate:   %d", meta.ReleaseDate)
	if meta.Cover != nil {
		t.Logf("Cover.Name:    %s", meta.Cover.Name)
		t.Logf("Cover.Key:     %s", meta.Cover.Key)
	}
	if meta.Poster != nil {
		t.Logf("Poster.Name:   %s", meta.Poster.Name)
		t.Logf("Poster.Key:    %s", meta.Poster.Key)
	}
	t.Logf("SampleImages:  %d", len(meta.SampleImages))

	require.NotEmpty(t, meta.Number)
	require.NotEmpty(t, meta.Title)
	require.NotNil(t, meta.Cover, "cover should exist")
	require.NotEmpty(t, meta.Cover.Name, "cover URL should not be empty")
	require.NotEmpty(t, meta.Cover.Key, "cover image should be downloaded successfully")
}

func TestFetchType_GoHTTP(t *testing.T) {
	yamlURL, numberID, _ := fetchTestEnv(t)
	data := fetchRemoteYAML(t, yamlURL)
	t.Logf("Loaded %d bytes from %s", len(data), yamlURL)
	cli := client.MustNewClient()
	doSearch(t, data, cli, numberID)
}

func TestFetchType_Browser(t *testing.T) {
	yamlURL, numberID, waitSelector := fetchTestEnv(t)
	goHTTPData := fetchRemoteYAML(t, yamlURL)
	browserData := toBrowserYAML(t, goHTTPData, waitSelector)
	t.Logf("Loaded %d bytes from %s, converted to browser YAML (%d bytes)", len(goHTTPData), yamlURL, len(browserData))
	t.Logf("Browser YAML:\n%s", string(browserData))

	baseCli := client.MustNewClient()
	nav := browser.NewNavigator(&browser.Config{
		DataDir: "/tmp/yamdc-test-browser",
	})
	t.Cleanup(func() { _ = nav.Close() })
	cli := browser.NewHTTPClient(baseCli, nav)
	doSearch(t, browserData, cli, numberID)
}
