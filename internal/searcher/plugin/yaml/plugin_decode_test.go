package yaml

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func TestApplyStringTransforms(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		transforms []*TransformSpec
		expect     string
	}{
		{name: "trim", value: " abc ", transforms: []*TransformSpec{{Kind: "trim"}}, expect: "abc"},
		{name: "trim_prefix", value: "abc123", transforms: []*TransformSpec{{Kind: "trim_prefix", Value: "abc"}}, expect: "123"},
		{name: "trim_suffix", value: "abc123", transforms: []*TransformSpec{{Kind: "trim_suffix", Value: "123"}}, expect: "abc"},
		{name: "trim_charset", value: ":abc:", transforms: []*TransformSpec{{Kind: "trim_charset", Cutset: ":"}}, expect: "abc"},
		{name: "replace", value: "a-b-c", transforms: []*TransformSpec{{Kind: "replace", Old: "-", New: "_"}}, expect: "a_b_c"},
		{name: "regex_extract", value: "abc-123", transforms: []*TransformSpec{{Kind: "regex_extract", Value: `(\d+)`, Index: 1}}, expect: "123"},
		{name: "regex_extract_bad", value: "abc", transforms: []*TransformSpec{{Kind: "regex_extract", Value: `[invalid`, Index: 0}}, expect: ""},
		{name: "regex_extract_out_of_range", value: "abc", transforms: []*TransformSpec{{Kind: "regex_extract", Value: `(abc)`, Index: 5}}, expect: ""},
		{name: "split_index", value: "a/b/c", transforms: []*TransformSpec{{Kind: "split_index", Sep: "/", Index: 1}}, expect: "b"},
		{name: "split_index_out_of_range", value: "a/b", transforms: []*TransformSpec{{Kind: "split_index", Sep: "/", Index: 5}}, expect: ""},
		{name: "to_upper", value: "abc", transforms: []*TransformSpec{{Kind: "to_upper"}}, expect: "ABC"},
		{name: "to_lower", value: "ABC", transforms: []*TransformSpec{{Kind: "to_lower"}}, expect: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyStringTransforms(tt.value, tt.transforms)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// --- applyOneListTransform ---

func TestApplyOneListTransform(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		spec   *TransformSpec
		expect []string
	}{
		{name: "remove_empty", input: []string{"a", "", "b", "  "}, spec: &TransformSpec{Kind: "remove_empty"}, expect: []string{"a", "b"}},
		{name: "dedupe", input: []string{"a", "b", "a"}, spec: &TransformSpec{Kind: "dedupe"}, expect: []string{"a", "b"}},
		{name: "map_trim", input: []string{" a ", " b "}, spec: &TransformSpec{Kind: "map_trim"}, expect: []string{"a", "b"}},
		{name: "replace", input: []string{"a-b", "c-d"}, spec: &TransformSpec{Kind: "replace", Old: "-", New: "_"}, expect: []string{"a_b", "c_d"}},
		{name: "split", input: []string{"a,b", "c"}, spec: &TransformSpec{Kind: "split", Sep: ","}, expect: []string{"a", "b", "c"}},
		{name: "to_upper", input: []string{"abc"}, spec: &TransformSpec{Kind: "to_upper"}, expect: []string{"ABC"}},
		{name: "to_lower", input: []string{"ABC"}, spec: &TransformSpec{Kind: "to_lower"}, expect: []string{"abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyOneListTransform(append([]string(nil), tt.input...), tt.spec)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// --- parseDurationMMSS ---

func TestParseDurationMMSS(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int64
	}{
		{name: "valid", input: "02:30", expect: 150},
		{name: "invalid_parts", input: "abc", expect: 0},
		{name: "bad_minutes", input: "xx:30", expect: 0},
		{name: "bad_seconds", input: "02:xx", expect: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDurationMMSS(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// --- parseDurationByKind ---

func TestParseDurationByKind(t *testing.T) {
	ctx := context.Background()
	assert.EqualValues(t, 120*60, parseDurationByKind(ctx, "duration_default", "120分钟"))
	assert.EqualValues(t, 3661, parseDurationByKind(ctx, "duration_hhmmss", "01:01:01"))
	assert.EqualValues(t, 120*60, parseDurationByKind(ctx, "duration_mm", "120"))
	assert.EqualValues(t, 3661, parseDurationByKind(ctx, "duration_human", "1h1m1s"))
	assert.EqualValues(t, 150, parseDurationByKind(ctx, "duration_mmss", "02:30"))
	assert.EqualValues(t, 0, parseDurationByKind(ctx, "unknown_kind", "120"))
}

// --- assignStringField / assignListField ---

func TestAssignStringField(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		field   string
		value   string
		parser  ParserSpec
		wantErr bool
	}{
		{name: "string_default", field: "number", value: "ABC", parser: ParserSpec{}},
		{name: "string_explicit", field: "title", value: "T", parser: ParserSpec{Kind: "string"}},
		{name: "date_only", field: "release_date", value: "2024-01-02", parser: ParserSpec{Kind: "date_only"}},
		{name: "duration_default", field: "duration", value: "120分钟", parser: ParserSpec{Kind: "duration_default"}},
		{name: "time_format", field: "release_date", value: "2024-01-02", parser: ParserSpec{Kind: "time_format", Layout: "2006-01-02"}},
		{name: "date_layout_soft", field: "release_date", value: "2024-01-02", parser: ParserSpec{Kind: "date_layout_soft", Layout: "2006-01-02"}},
		{name: "unsupported_parser", field: "title", value: "T", parser: ParserSpec{Kind: "bad"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mv := &model.MovieMeta{}
			err := assignStringField(ctx, mv, tt.field, tt.value, tt.parser)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAssignListField(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	err := assignListField(ctx, mv, "actors", []string{"A", "B"}, ParserSpec{})
	require.NoError(t, err)
	assert.Equal(t, []string{"A", "B"}, mv.Actors)

	err = assignListField(ctx, mv, "genres", []string{"G"}, ParserSpec{Kind: "string_list"})
	require.NoError(t, err)
	assert.Equal(t, []string{"G"}, mv.Genres)

	mv2 := &model.MovieMeta{}
	err = assignListField(ctx, mv2, "sample_images", []string{"url1"}, ParserSpec{})
	require.NoError(t, err)
	require.Len(t, mv2.SampleImages, 1)

	err = assignListField(ctx, mv, "actors", []string{"A"}, ParserSpec{Kind: "bad"})
	require.Error(t, err)
}

func TestAssignStringFieldByName(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	assignStringFieldByName(mv, "number", "N")
	assignStringFieldByName(mv, "title", "T")
	assignStringFieldByName(mv, "plot", "P")
	assignStringFieldByName(mv, "studio", "S")
	assignStringFieldByName(mv, "label", "L")
	assignStringFieldByName(mv, "director", "D")
	assignStringFieldByName(mv, "series", "R")
	assignStringFieldByName(mv, "cover", "C")
	assignStringFieldByName(mv, "poster", "PS")
	assert.Equal(t, "N", mv.Number)
	assert.Equal(t, "T", mv.Title)
	assert.Equal(t, "D", mv.Director)
	assert.Equal(t, "C", mv.Cover.Name)
	assert.Equal(t, "PS", mv.Poster.Name)
}

// --- normalizeLang ---

func TestAssignDateField(t *testing.T) {
	mv := &model.MovieMeta{}
	err := assignDateField(mv, "release_date", ParserSpec{Kind: "time_format", Layout: "2006-01-02"}, "2024-05-06")
	require.NoError(t, err)
	assert.NotZero(t, mv.ReleaseDate)

	mv2 := &model.MovieMeta{}
	err = assignDateField(mv2, "release_date", ParserSpec{Kind: "time_format", Layout: "2006-01-02"}, "bad")
	require.Error(t, err)

	mv3 := &model.MovieMeta{}
	err = assignDateField(mv3, "release_date", ParserSpec{Kind: "date_layout_soft", Layout: "2006-01-02"}, "2024-05-06")
	require.NoError(t, err)
	assert.NotZero(t, mv3.ReleaseDate)
}

// --- checkBaseResponseStatus ---

func TestApplyPostprocess(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
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
postprocess:
  assign:
    title: "${meta.title} (edited)"
  defaults:
    title_lang: ja
    plot_lang: en
    genres_lang: zh-cn
    actors_lang: zh-tw
  switch_config:
    disable_release_date_check: true
    disable_number_replace: true
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	mv := &model.MovieMeta{Title: "Original", Cover: &model.File{}, Poster: &model.File{}}
	plg.applyPostprocess(ctx, mv)
	assert.Contains(t, mv.Title, "edited")
	assert.True(t, mv.SwithConfig.DisableReleaseDateCheck)
	assert.True(t, mv.SwithConfig.DisableNumberReplace)
}

// --- buildRequest with various body types ---

func TestAssignListField_Unsupported(t *testing.T) {
	err := assignListField(context.Background(), nil, "actors", []string{"a"}, ParserSpec{Kind: "custom"})
	require.ErrorIs(t, err, errUnsupportedListParser)
}

func TestAssignStringField_Unsupported(t *testing.T) {
	err := assignStringField(context.Background(), nil, "title", "t", ParserSpec{Kind: "bogus"})
	require.ErrorIs(t, err, errUnsupportedParser)
}

func TestApplyPostprocess_SwitchConfig(t *testing.T) {
	spec := simpleOneStepSpec("https://example.com")
	spec.Postprocess = &PostprocessSpec{
		SwitchConfig: &SwitchConfigSpec{
			DisableReleaseDateCheck: true,
			DisableNumberReplace:    true,
		},
	}
	plg := buildTestPlugin(t, spec)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	data := []byte(`<html><body><h1 class="title">T</h1><div class="actors"><span>A</span></div></body></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, mv.SwithConfig.DisableReleaseDateCheck)
	assert.True(t, mv.SwithConfig.DisableNumberReplace)
}

func TestParseDurationMMSS_Extended(t *testing.T) {
	tests := []struct {
		input  string
		expect int64
	}{
		{"1:30", 90},
		{"0:00", 0},
		{"invalid", 0},
		{"a:b", 0},
		{"10:x", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expect, parseDurationMMSS(tt.input))
		})
	}
}

func TestDecodeHTML_RequiredListEmpty(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"actors": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//span[@class='actor']/text()"}, Required: true},
		}},
	}
	plg := buildTestPlugin(t, spec)
	node := helperParseHTMLStr(t, `<html><body><p>no actors here</p></body></html>`)
	mv, err := plg.decodeHTML(context.Background(), node)
	require.NoError(t, err)
	assert.Nil(t, mv)
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
		}},
	}
	plg := buildTestPlugin(t, spec)
	_, err := plg.decodeJSON(context.Background(), []byte(`not-json`))
	require.Error(t, err)
}

func TestDecodeJSON_RequiredListEmpty(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"actors": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.actors"}, Required: true},
		}},
	}
	plg := buildTestPlugin(t, spec)
	mv, err := plg.decodeJSON(context.Background(), []byte(`{"title":"T"}`))
	require.NoError(t, err)
	assert.Nil(t, mv)
}

func TestDecodeJSON_RequiredStringEmpty(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}, Required: true},
		}},
	}
	plg := buildTestPlugin(t, spec)
	mv, err := plg.decodeJSON(context.Background(), []byte(`{"other":"value"}`))
	require.NoError(t, err)
	assert.Nil(t, mv)
}
