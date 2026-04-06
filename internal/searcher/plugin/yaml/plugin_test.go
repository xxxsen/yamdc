package yaml

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
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/store"
)

func TestTemplateValidationRejectsUnknownFunction(t *testing.T) {
	_, err := compileTemplate(`${unknown_fn(${number})}`)
	require.Error(t, err)
}

func TestSyncBundleRemovesObsoletePlugins(t *testing.T) {
	const (
		oldName = "__bundle_test_old__"
		newName = "__bundle_test_new__"
	)
	orig := factory.NewRegisterContext()
	for _, name := range factory.Plugins() {
		cr, ok := factory.Lookup(name)
		require.True(t, ok)
		orig.Register(name, cr)
	}
	defer factory.Swap(orig)
	SyncBundle(map[string][]byte{
		oldName: []byte(`
version: 1
name: __bundle_test_old__
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
      required: true
`),
	})
	_, ok := factory.Lookup(oldName)
	require.True(t, ok)

	SyncBundle(map[string][]byte{
		newName: []byte(`
version: 1
name: __bundle_test_new__
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
      required: true
`),
	})
	_, ok = factory.Lookup(oldName)
	require.False(t, ok)
	_, ok = factory.Lookup(newName)
	require.True(t, ok)

	SyncBundle(nil)
	_, ok = factory.Lookup(newName)
	require.False(t, ok)
}

func TestYAML_Jav321_OneStep(t *testing.T) {
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
  <b>出演者</b><span class="actors"><a href="/star/a">Alice</a><a href="/star/b">Bob</a></span>
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

	plg := mustPluginFromYAML(t, strings.ReplaceAll(oneStepFixtureYAML(), "https://fixture.example", srv.URL))
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

func TestYAML_JavDB_TwoStep(t *testing.T) {
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

	plg := mustPluginFromYAML(t, strings.ReplaceAll(twoStepFixtureYAML(), "https://fixture.example", srv.URL))
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

func TestYAML_Airav_JSON(t *testing.T) {
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/img/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/video/barcode/ABC-123", r.URL.Path)
		require.Equal(t, "zh-TW", r.URL.Query().Get("lng"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.ReplaceAll(`{
  "count": 1,
  "status": "ok",
  "result": {
    "barcode": "ABC-123",
    "name": "Airav Title",
    "description": "Airav Plot",
    "img_url": "{{HOST}}/img/cover.jpg",
    "publish_date": "2024-05-06",
    "actors": [{"name": "Alice"}, {"name": "Bob"}],
    "images": ["{{HOST}}/img/1.jpg", "{{HOST}}/img/2.jpg"],
    "tags": [{"name": "Drama"}, {"name": "Action"}],
    "factories": [{"name": "Studio A"}]
  }
}`, "{{HOST}}", baseURL)))
	}))
	defer srv.Close()
	baseURL = srv.URL

	plg := mustPluginFromYAML(t, strings.ReplaceAll(jsonFixtureYAML(), "https://fixture.example", srv.URL))
	meta := mustSearch(t, "airav", plg, srv.Client(), "ABC-123")
	require.Equal(t, "ABC-123", meta.Number)
	require.Equal(t, "Airav Title", meta.Title)
	require.Equal(t, "Airav Plot", meta.Plot)
	require.Equal(t, []string{"Alice", "Bob"}, meta.Actors)
	require.Equal(t, "Studio A", meta.Studio)
	require.Equal(t, []string{"Drama", "Action"}, meta.Genres)
	require.Equal(t, srv.URL+"/img/cover.jpg", meta.Cover.Name)
	require.Len(t, meta.SampleImages, 2)
	require.NotZero(t, meta.ReleaseDate)
}

func mustPluginFromYAML(t *testing.T, data string) *YAMLSearchPlugin {
	t.Helper()
	plg, err := NewFromBytes([]byte(data))
	require.NoError(t, err)
	out, ok := plg.(*YAMLSearchPlugin)
	require.True(t, ok)
	return out
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

func oneStepFixtureYAML() string {
	return `
version: 1
name: fixture-one-step
type: one-step
hosts:
  - https://fixture.example
request:
  method: POST
  path: /search
  body:
    kind: form
    values:
      sn: ${number}
scrape:
  format: html
  fields:
    number:
      selector:
        kind: xpath
        expr: //b[contains(text(),"品番")]/following-sibling::node()[1]
      transforms:
        - kind: trim_charset
          cutset: ": \t"
      parser: string
      required: true
    title:
      selector:
        kind: xpath
        expr: /html/body/div[2]/div[1]/div[1]/div[1]/h3/text()
      parser: string
      required: true
    plot:
      selector:
        kind: xpath
        expr: /html/body/div[2]/div[1]/div[1]/div[3]/div/text()
      parser: string
    actors:
      selector:
        kind: xpath
        expr: //span[@class="actors"]/a/text()
        multi: true
      parser: string_list
    release_date:
      selector:
        kind: xpath
        expr: //b[contains(text(),"配信開始日")]/following-sibling::node()[1]
      transforms:
        - kind: trim_charset
          cutset: ": \t"
      parser:
        kind: time_format
        layout: "2006-01-02"
    duration:
      selector:
        kind: xpath
        expr: //b[contains(text(),"収録時間")]/following-sibling::node()[1]
      transforms:
        - kind: trim_charset
          cutset: ": \t"
      parser: duration_default
    studio:
      selector:
        kind: xpath
        expr: //b[contains(text(),"メーカー")]/following-sibling::a[1]/text()
      parser: string
    label:
      selector:
        kind: xpath
        expr: //b[contains(text(),"メーカー")]/following-sibling::a[1]/text()
      parser: string
    series:
      selector:
        kind: xpath
        expr: //b[contains(text(),"シリーズ")]/following-sibling::node()[1]
      transforms:
        - kind: trim_charset
          cutset: ": \t"
      parser: string
    genres:
      selector:
        kind: xpath
        expr: //b[contains(text(),"ジャンル")]/following-sibling::a/text()
        multi: true
      parser: string_list
    cover:
      selector:
        kind: xpath
        expr: /html/body/div[2]/div[2]/div[1]/p/a/img/@src
      parser: string
    sample_images:
      selector:
        kind: xpath
        expr: //div[@class="col-md-3"]/div[@class="col-xs-12 col-md-12"]/p/a/img/@src
        multi: true
      parser: string_list
postprocess:
  defaults:
    title_lang: ja
    plot_lang: ja
`
}

func twoStepFixtureYAML() string {
	return `
version: 1
name: fixture-two-step
type: two-step
hosts:
  - https://fixture.example
request:
  method: GET
  path: /search
  query:
    q: ${number}
workflow:
  search_select:
    selectors:
      - name: read_link
        kind: xpath
        expr: //div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"]/a/@href
      - name: read_code
        kind: xpath
        expr: //div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"]/a/div[@class="video-title"]/strong/text()
    match:
      mode: and
      conditions:
        - equals("${clean_number(${item.read_code})}", "${clean_number(${number})}")
      expect_count: 1
    return: ${item.read_link}
    next_request:
      method: GET
      url: ${build_url(${host}, ${value})}
scrape:
  format: html
  fields:
    number:
      selector:
        kind: xpath
        expr: //a[@class="button is-white copy-to-clipboard"]/@data-clipboard-text
      parser: string
      required: true
    title:
      selector:
        kind: xpath
        expr: //h2[@class="title is-4"]/strong[@class="current-title"]/text()
      parser: string
      required: true
    actors:
      selector:
        kind: xpath
        expr: //strong[contains(text(),"演員")]/following-sibling::span[@class="value"]/a/text()
        multi: true
      parser: string_list
    release_date:
      selector:
        kind: xpath
        expr: //strong[contains(text(),"日期")]/following-sibling::span[@class="value"]/text()
      parser:
        kind: time_format
        layout: "2006-01-02"
    duration:
      selector:
        kind: xpath
        expr: //strong[contains(text(),"時長")]/following-sibling::span[@class="value"]/text()
      parser: duration_default
    studio:
      selector:
        kind: xpath
        expr: //strong[contains(text(),"片商")]/following-sibling::span[@class="value"]/text()
      parser: string
    series:
      selector:
        kind: xpath
        expr: //strong[contains(text(),"系列")]/following-sibling::span[@class="value"]/text()
      parser: string
    genres:
      selector:
        kind: xpath
        expr: //strong[contains(text(),"類別")]/following-sibling::span[@class="value"]/a/text()
        multi: true
      parser: string_list
    cover:
      selector:
        kind: xpath
        expr: //div[@class="column column-video-cover"]/a/img/@src
      parser: string
    sample_images:
      selector:
        kind: xpath
        expr: //div[@class="tile-images preview-images"]/a[@class="tile-item"]/@href
        multi: true
      parser: string_list
postprocess:
  defaults:
    title_lang: ja
`
}

func jsonFixtureYAML() string {
	return `
version: 1
name: fixture-json
type: one-step
hosts:
  - https://fixture.example
request:
  method: GET
  path: /api/video/barcode/${number}
  query:
    lng: zh-TW
scrape:
  format: json
  fields:
    number:
      selector:
        kind: jsonpath
        expr: $.result.barcode
      parser: string
      required: true
    title:
      selector:
        kind: jsonpath
        expr: $.result.name
      parser: string
      required: true
    plot:
      selector:
        kind: jsonpath
        expr: $.result.description
      parser: string
    actors:
      selector:
        kind: jsonpath
        expr: $.result.actors[*].name
        multi: true
      parser: string_list
    studio:
      selector:
        kind: jsonpath
        expr: $.result.factories[0].name
      parser: string
    genres:
      selector:
        kind: jsonpath
        expr: $.result.tags[*].name
        multi: true
      parser: string_list
    cover:
      selector:
        kind: jsonpath
        expr: $.result.img_url
      parser: string
    sample_images:
      selector:
        kind: jsonpath
        expr: $.result.images[*]
        multi: true
      parser: string_list
    release_date:
      selector:
        kind: jsonpath
        expr: $.result.publish_date
      parser:
        kind: time_format
        layout: "2006-01-02"
`
}
