package yamlplugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/searcher"
	yamlassets "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"github.com/xxxsen/yamdc/internal/store"
)

func TestTemplateValidationRejectsUnknownFunction(t *testing.T) {
	_, err := compileTemplate(`${unknown_fn(${number})}`)
	require.Error(t, err)
}

func TestYAMLPlugin_Jav321_OneStep(t *testing.T) {
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/img/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/search", r.URL.Path)
		require.NoError(t, r.ParseForm())
		require.Equal(t, "ABC-123", r.Form.Get("sn"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(strings.ReplaceAll(`
<html><body>
  <div></div>
  <div>
    <div>
      <div>
        <div><h3>Sample Title</h3></div>
        <div></div>
        <div><div>Plot Value</div></div>
      </div>
    </div>
    <div>
      <div>
        <p><a><img src="{{HOST}}/img/cover.jpg"></a></p>
      </div>
    </div>
  </div>
  <b>品番</b>: ABC-123
  <b>出演者</b><a href="/star/a">Alice</a><a href="/star/b">Bob</a>
  <b>配信開始日</b>: 2024-01-02
  <b>収録時間</b>: 120 分钟
  <b>メーカー</b><a href="/company/a">Studio A</a>
  <b>シリーズ</b>: Series X
  <b>ジャンル</b><a href="/genre/a">Drama</a><a href="/genre/b">Tag2</a>
  <div class="col-md-3"><div class="col-xs-12 col-md-12"><p><a><img src="{{HOST}}/img/1.jpg"></a></p></div></div>
  <div class="col-md-3"><div class="col-xs-12 col-md-12"><p><a><img src="{{HOST}}/img/2.jpg"></a></p></div></div>
</body></html>`, "{{HOST}}", baseURL)))
	}))
	defer srv.Close()
	baseURL = srv.URL

	plg := mustPluginFromBuiltinYAML(t, "jav321", map[string]string{"https://www.jav321.com": srv.URL})
	meta := mustSearch(t, "jav321", plg, srv.Client(), "ABC-123")
	require.Equal(t, "ABC-123", meta.Number)
	require.Equal(t, "Sample Title", meta.Title)
	require.Equal(t, []string{"Alice", "Bob"}, meta.Actors)
	require.Equal(t, "Studio A", meta.Studio)
	require.Equal(t, "Studio A", meta.Label)
	require.Equal(t, "Series X", meta.Series)
	require.Equal(t, []string{"Drama", "Tag2"}, meta.Genres)
	require.Equal(t, srv.URL+"/img/cover.jpg", meta.Cover.Name)
	require.Len(t, meta.SampleImages, 2)
	require.Equal(t, enum.MetaLangJa, meta.TitleLang)
	require.Equal(t, enum.MetaLangJa, meta.PlotLang)
	require.NotZero(t, meta.ReleaseDate)
	require.EqualValues(t, 120*60, meta.Duration)
}

func TestYAMLPlugin_JavDB_TwoStep(t *testing.T) {
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/img/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		switch {
		case r.URL.Path == "/search":
			require.Equal(t, "ABC-123", r.URL.Query().Get("q"))
			_, _ = w.Write([]byte(`
<html><body>
  <div class="movie-list h cols-4 vcols-8">
    <div class="item">
      <a href="/v/target">
        <div class="video-title"><strong>ABC123</strong></div>
      </a>
    </div>
    <div class="item">
      <a href="/v/other">
        <div class="video-title"><strong>ZZZ999</strong></div>
      </a>
    </div>
  </div>
</body></html>`))
		case r.URL.Path == "/v/target":
			_, _ = w.Write([]byte(strings.ReplaceAll(`
<html><body>
  <a class="button is-white copy-to-clipboard" data-clipboard-text="ABC-123"></a>
  <h2 class="title is-4"><strong class="current-title">JavDB Title</strong></h2>
  <div><strong>演員</strong><span class="value"><a>Alice</a><a>Bob</a></span></div>
  <div><strong>日期</strong><span class="value">2024-05-06</span></div>
  <div><strong>時長</strong><span class="value">150 分钟</span></div>
  <div><strong>片商</strong><span class="value">Studio J</span></div>
  <div><strong>系列</strong><span class="value">Series J</span></div>
  <div><strong>類別</strong><span class="value"><a>Drama</a><a>Action</a></span></div>
  <div class="column column-video-cover"><a><img src="{{HOST}}/img/javdb-cover.jpg"></a></div>
  <div class="tile-images preview-images"><a class="tile-item" href="{{HOST}}/img/1.jpg"></a><a class="tile-item" href="{{HOST}}/img/2.jpg"></a></div>
</body></html>`, "{{HOST}}", baseURL)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	baseURL = srv.URL

	plg := mustPluginFromBuiltinYAML(t, "javdb", map[string]string{"https://javdb.com": srv.URL})
	meta := mustSearch(t, "javdb", plg, srv.Client(), "ABC-123")
	require.Equal(t, "ABC-123", meta.Number)
	require.Equal(t, "JavDB Title", meta.Title)
	require.Equal(t, []string{"Alice", "Bob"}, meta.Actors)
	require.Equal(t, "Studio J", meta.Studio)
	require.Equal(t, "Series J", meta.Series)
	require.Equal(t, []string{"Drama", "Action"}, meta.Genres)
	require.Equal(t, srv.URL+"/img/javdb-cover.jpg", meta.Cover.Name)
	require.Len(t, meta.SampleImages, 2)
	require.Equal(t, enum.MetaLangJa, meta.TitleLang)
	require.NotZero(t, meta.ReleaseDate)
	require.EqualValues(t, 150*60, meta.Duration)
}

func mustPluginFromYAML(t *testing.T, data string) *YAMLSearchPlugin {
	t.Helper()
	plg, err := NewFromBytes([]byte(data))
	require.NoError(t, err)
	out, ok := plg.(*YAMLSearchPlugin)
	require.True(t, ok)
	return out
}

func mustPluginFromBuiltinYAML(t *testing.T, name string, replacements map[string]string) *YAMLSearchPlugin {
	t.Helper()
	data, err := yamlassets.ReadFile(name + ".yaml")
	require.NoError(t, err)
	raw := string(data)
	for old, newValue := range replacements {
		raw = strings.ReplaceAll(raw, old, newValue)
	}
	return mustPluginFromYAML(t, raw)
}

func mustSearch(t *testing.T, name string, plg *YAMLSearchPlugin, cli *http.Client, numberID string) *model.MovieMeta {
	t.Helper()
	s, err := searcher.NewDefaultSearcher(name, plg, searcher.WithHTTPClient(cli), searcher.WithStorage(store.NewMemStorage()), searcher.WithSearchCache(false))
	require.NoError(t, err)
	num, err := number.Parse(numberID)
	require.NoError(t, err)
	meta, ok, err := s.Search(context.Background(), num)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, meta)
	return meta
}
