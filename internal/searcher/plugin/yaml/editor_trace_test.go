package yaml

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
)

func TestIsListField(t *testing.T) {
	assert.True(t, isListField("actors"))
	assert.True(t, isListField("genres"))
	assert.True(t, isListField("sample_images"))
	assert.False(t, isListField("title"))
}

// --- movieMetaStringMap ---

func TestTraceAssignStringField(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	result, err := traceAssignStringField(ctx, mv, "title", "MyTitle", ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "MyTitle", result)

	mv2 := &model.MovieMeta{}
	result, err = traceAssignStringField(ctx, mv2, "title", "", ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "", result)

	mv3 := &model.MovieMeta{}
	result, err = traceAssignStringField(ctx, mv3, "release_date", "2024-01-02", ParserSpec{Kind: "date_only"})
	require.NoError(t, err)
	assert.NotZero(t, result)

	mv4 := &model.MovieMeta{}
	result, err = traceAssignStringField(ctx, mv4, "duration", "120分钟", ParserSpec{Kind: "duration_default"})
	require.NoError(t, err)
	assert.NotZero(t, result)

	mv5 := &model.MovieMeta{}
	_, err = traceAssignStringField(ctx, mv5, "title", "T", ParserSpec{Kind: "unknown_custom"})
	require.Error(t, err)
}

// --- traceStringTransforms / traceListTransforms ---

func TestTraceStringTransforms(t *testing.T) {
	var steps []TransformStep
	result := traceStringTransforms(" abc ", []*TransformSpec{{Kind: "trim"}}, &steps)
	assert.Equal(t, "abc", result)
	assert.Len(t, steps, 1)
}

func TestTraceListTransforms(t *testing.T) {
	var steps []TransformStep
	result := traceListTransforms([]string{" a ", " b "}, []*TransformSpec{{Kind: "map_trim"}}, &steps)
	assert.Equal(t, []string{"a", "b"}, result)
	assert.Len(t, steps, 1)
}

// --- captureHTTPResponse ---

func TestFieldByName_NoMatch(t *testing.T) {
	plg := compileTestPlugin(t, simpleOneStepSpec("https://example.com"))
	f := plg.fieldByName("nonexistent_field")
	assert.Nil(t, f)
}

func TestTraceAssignStringField_DateParser(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	result, err := traceAssignStringField(ctx, mv, "release_date", "2024-01-15",
		ParserSpec{Kind: "time_format", Layout: "2006-01-02"})
	require.NoError(t, err)
	assert.NotZero(t, result)
}

func TestTraceAssignStringField_DurationMMSS(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	result, err := traceAssignStringField(ctx, mv, "duration", "1:30",
		ParserSpec{Kind: "duration_mmss"})
	require.NoError(t, err)
	assert.NotZero(t, result)
}

func TestTraceAssignStringField_UnknownParserDefault(t *testing.T) {
	ctx := context.Background()
	mv := &model.MovieMeta{}
	result, err := traceAssignStringField(ctx, mv, "title", "T",
		ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "T", result)
}

func TestTraceFieldJSON_ListField(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"actors": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.actors"}},
		}},
	}
	plg := buildTestPluginFrom(t, spec)
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	var doc any
	_ = json.Unmarshal([]byte(`{"actors":["Alice","Bob"]}`), &doc)
	field := plg.fieldByName("actors")
	dbg, err := plg.traceFieldJSON(context.Background(), mv, doc, field)
	require.NoError(t, err)
	assert.True(t, dbg.Matched)
}

func TestTraceFieldJSON_StringFieldEmpty(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
		}},
	}
	plg := buildTestPluginFrom(t, spec)
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	var doc any
	_ = json.Unmarshal([]byte(`{"other":"value"}`), &doc)
	field := plg.fieldByName("title")
	dbg, err := plg.traceFieldJSON(context.Background(), mv, doc, field)
	require.NoError(t, err)
	assert.False(t, dbg.Matched)
}

func TestTraceAssignStringField_DateOnly(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "release_date", "2024-01-15", ParserSpec{Kind: "date_only"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTraceAssignStringField_DurationMmss(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "duration", "120:30", ParserSpec{Kind: "duration_mmss"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTraceAssignStringField_EmptyValue(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "title", "", ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestTraceAssignStringField_UnknownParserErrors(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	_, err := traceAssignStringField(context.Background(), mv, "title", "Test", ParserSpec{Kind: "unknown_parser"})
	require.Error(t, err)
}

func TestTraceAssignStringField_StringKind(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "title", "Test", ParserSpec{Kind: "string"})
	require.NoError(t, err)
	assert.Equal(t, "Test", result)
}

func TestTraceAssignStringField_NoParserKind(t *testing.T) {
	mv := &model.MovieMeta{Cover: &model.File{}, Poster: &model.File{}}
	result, err := traceAssignStringField(context.Background(), mv, "title", "Test", ParserSpec{})
	require.NoError(t, err)
	assert.Equal(t, "Test", result)
}

func TestTraceDecodeHTML_RequiredFieldNotMatched(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//h1/text()"}, Required: true},
		}},
	}
	plg := buildTestPluginFrom(t, spec)
	node := helperParseHTMLForEditor(t, "<html><body><p>no title</p></body></html>")
	fields := map[string]FieldDebugResult{}
	mv, err := plg.traceDecodeHTML(context.Background(), node, fields)
	require.NoError(t, err)
	assert.Nil(t, mv)
}

func TestTraceDecodeJSON_RequiredFieldNotMatched(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
		Scrape: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}, Required: true},
		}},
	}
	plg := buildTestPluginFrom(t, spec)
	fields := map[string]FieldDebugResult{}
	mv, err := plg.traceDecodeJSON(context.Background(), []byte(`{"other":"val"}`), fields)
	require.NoError(t, err)
	assert.Nil(t, mv)
}
