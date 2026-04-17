package yaml

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/browser"
	"github.com/xxxsen/yamdc/internal/flarerr"
	"github.com/xxxsen/yamdc/internal/model"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

// --- template tests ---

func TestTemplateRender(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		ctx     *evalContext
		expect  string
		wantErr bool
	}{
		{name: "plain_text", raw: "hello", ctx: &evalContext{}, expect: "hello"},
		{name: "number_var", raw: "/search/${number}", ctx: &evalContext{number: "ABC-123"}, expect: "/search/ABC-123"},
		{name: "host_var", raw: "${host}/path", ctx: &evalContext{host: "https://example.com"}, expect: "https://example.com/path"},
		{name: "body_var", raw: "${body}", ctx: &evalContext{body: "content"}, expect: "content"},
		{name: "value_var", raw: "${value}", ctx: &evalContext{value: "val"}, expect: "val"},
		{name: "candidate_var", raw: "${candidate}", ctx: &evalContext{candidate: "cand"}, expect: "cand"},
		{name: "vars_ref", raw: "${vars.myvar}", ctx: &evalContext{vars: map[string]string{"myvar": "v"}}, expect: "v"},
		{name: "item_ref", raw: "${item.col}", ctx: &evalContext{item: map[string]string{"col": "v"}}, expect: "v"},
		{name: "item_variables_ref", raw: "${item_variables.x}", ctx: &evalContext{itemVariables: map[string]string{"x": "y"}}, expect: "y"},
		{name: "meta_ref", raw: "${meta.title}", ctx: &evalContext{meta: map[string]string{"title": "T"}}, expect: "T"},
		{name: "to_upper", raw: "${to_upper(${number})}", ctx: &evalContext{number: "abc"}, expect: "ABC"},
		{name: "to_lower", raw: "${to_lower(${number})}", ctx: &evalContext{number: "ABC"}, expect: "abc"},
		{name: "trim", raw: "${trim(\" hello \")}", ctx: &evalContext{}, expect: "hello"},
		{name: "trim_prefix", raw: "${trim_prefix(\"abc123\", \"abc\")}", ctx: &evalContext{}, expect: "123"},
		{name: "trim_suffix", raw: "${trim_suffix(\"abc123\", \"123\")}", ctx: &evalContext{}, expect: "abc"},
		{name: "replace", raw: "${replace(\"a-b-c\", \"-\", \"_\")}", ctx: &evalContext{}, expect: "a_b_c"},
		{name: "clean_number", raw: "${clean_number(\"ABC-123\")}", ctx: &evalContext{}, expect: "ABC123"},
		{name: "concat", raw: "${concat(\"a\", \"b\", \"c\")}", ctx: &evalContext{}, expect: "abc"},
		{name: "first_non_empty", raw: "${first_non_empty(\"\", \"b\")}", ctx: &evalContext{}, expect: "b"},
		{name: "first_non_empty_all_empty", raw: "${first_non_empty(\"\", \"\")}", ctx: &evalContext{}, expect: ""},
		{name: "last_segment", raw: "${last_segment(\"a/b/c\", \"/\")}", ctx: &evalContext{}, expect: "c"},
		{name: "last_segment_empty_sep", raw: "${last_segment(\"abc\", \"\")}", ctx: &evalContext{}, expect: "abc"},
		{name: "build_url", raw: "${build_url(\"https://example.com\", \"/path\")}", ctx: &evalContext{}, expect: "https://example.com/path"},
		{name: "build_url_abs_ref", raw: "${build_url(\"https://example.com\", \"https://other.com/x\")}", ctx: &evalContext{}, expect: "https://other.com/x"},
		{name: "unknown_var", raw: "${unknown_var}", ctx: &evalContext{}, wantErr: true},
		{name: "nested_call", raw: "${to_upper(${to_lower(\"ABC\")})}", ctx: &evalContext{}, expect: "ABC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := compileTemplate(tt.raw)
			if err != nil {
				if tt.wantErr {
					return
				}
				require.NoError(t, err)
			}
			result, err := tmpl.Render(tt.ctx)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expect, result)
			}
		})
	}
}

func TestTemplateFunc_Errors(t *testing.T) {
	tests := []struct {
		name string
		fn   string
		args []string
	}{
		{"build_url_1_arg", "build_url", []string{"a"}},
		{"to_upper_0_arg", "to_upper", nil},
		{"to_lower_0_arg", "to_lower", nil},
		{"trim_0_arg", "trim", nil},
		{"trim_prefix_1_arg", "trim_prefix", []string{"a"}},
		{"trim_suffix_1_arg", "trim_suffix", []string{"a"}},
		{"replace_2_args", "replace", []string{"a", "b"}},
		{"clean_number_0_arg", "clean_number", nil},
		{"first_non_empty_1_arg", "first_non_empty", []string{"a"}},
		{"last_segment_1_arg", "last_segment", []string{"a"}},
		{"unknown_func", "nonexistent", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalTemplateFunc(tt.fn, tt.args)
			require.Error(t, err)
		})
	}
}

func TestUnterminatedTemplate(t *testing.T) {
	_, err := compileTemplate("${number")
	require.Error(t, err)
}

func TestIsIdentifier(t *testing.T) {
	assert.True(t, isIdentifier("abc"))
	assert.True(t, isIdentifier("_abc"))
	assert.True(t, isIdentifier("abc123"))
	assert.True(t, isIdentifier("abc.def"))
	assert.False(t, isIdentifier(""))
	assert.False(t, isIdentifier("123abc"))
	assert.False(t, isIdentifier("abc-def"))
}

func TestIsVariableRef(t *testing.T) {
	assert.True(t, isVariableRef("number"))
	assert.True(t, isVariableRef("host"))
	assert.True(t, isVariableRef("body"))
	assert.True(t, isVariableRef("value"))
	assert.True(t, isVariableRef("candidate"))
	assert.True(t, isVariableRef("vars.x"))
	assert.True(t, isVariableRef("item.x"))
	assert.True(t, isVariableRef("item_variables.x"))
	assert.True(t, isVariableRef("meta.x"))
	assert.False(t, isVariableRef("unknown"))
	assert.False(t, isVariableRef(""))
}

func TestParseCall(t *testing.T) {
	name, args, ok, err := parseCall("fn(a, b)")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "fn", name)
	assert.Equal(t, []string{"a", "b"}, args)

	_, _, ok, _ = parseCall("not_a_call") //nolint:dogsled
	assert.False(t, ok)

	_, _, ok, _ = parseCall("123(a)") //nolint:dogsled
	assert.False(t, ok)
}

func TestSplitArgs(t *testing.T) {
	args, err := splitArgs("")
	require.NoError(t, err)
	assert.Nil(t, args)

	args, err = splitArgs(`"a", "b"`)
	require.NoError(t, err)
	assert.Equal(t, []string{`"a"`, `"b"`}, args)

	_, err = splitArgs(`"unterminated`)
	require.Error(t, err)

	_, err = splitArgs(`extra)`)
	require.Error(t, err)
}

func TestSelectedHost(t *testing.T) {
	assert.Equal(t, "h", selectedHost(&evalContext{host: "h"}, nil))
	assert.Equal(t, "", selectedHost(nil, nil))
	assert.NotEmpty(t, selectedHost(nil, []string{"a"}))
}

func TestResolveMapRef(t *testing.T) {
	v, ok := resolveMapRef("vars.x", "vars.", map[string]string{"x": "y"})
	assert.True(t, ok)
	assert.Equal(t, "y", v)

	_, ok = resolveMapRef("other.x", "vars.", map[string]string{"x": "y"})
	assert.False(t, ok)

	_, ok = resolveMapRef("vars.x", "vars.", nil)
	assert.False(t, ok)
}

// --- condition tests ---

func TestCompileConditionGroup(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ConditionGroupSpec
		wantErr bool
		wantNil bool
	}{
		{name: "nil_spec", spec: nil, wantNil: true},
		{name: "invalid_mode", spec: &ConditionGroupSpec{Mode: "bad", Conditions: []string{"x"}}, wantErr: true},
		{name: "empty_conditions", spec: &ConditionGroupSpec{Mode: "and"}, wantErr: true},
		{name: "valid_and", spec: &ConditionGroupSpec{
			Mode:       "and",
			Conditions: []string{`contains("a", "a")`},
		}},
		{name: "valid_or", spec: &ConditionGroupSpec{
			Mode:       "or",
			Conditions: []string{`equals("a", "b")`},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := compileConditionGroup(tt.spec)
			switch {
			case tt.wantErr:
				require.Error(t, err)
			case tt.wantNil:
				require.NoError(t, err)
				require.Nil(t, g)
			default:
				require.NoError(t, err)
				require.NotNil(t, g)
			}
		})
	}
}

func TestConditionEval(t *testing.T) {
	ctx := &evalContext{number: "ABC-123"}
	tests := []struct {
		name   string
		raw    string
		expect bool
	}{
		{name: "contains_true", raw: `contains("ABC-123", "ABC")`, expect: true},
		{name: "contains_false", raw: `contains("ABC-123", "XYZ")`, expect: false},
		{name: "equals_true", raw: `equals("a", "a")`, expect: true},
		{name: "equals_false", raw: `equals("a", "b")`, expect: false},
		{name: "starts_with_true", raw: `starts_with("ABC-123", "ABC")`, expect: true},
		{name: "starts_with_false", raw: `starts_with("ABC-123", "XYZ")`, expect: false},
		{name: "ends_with_true", raw: `ends_with("ABC-123", "123")`, expect: true},
		{name: "ends_with_false", raw: `ends_with("ABC-123", "XYZ")`, expect: false},
		{name: "regex_match_true", raw: `regex_match("ABC-123", "^ABC")`, expect: true},
		{name: "regex_match_false", raw: `regex_match("ABC-123", "^XYZ")`, expect: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond, err := compileCondition(tt.raw)
			require.NoError(t, err)
			result, err := cond.Eval(ctx, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestConditionEval_SelectorExists(t *testing.T) {
	cond, err := compileCondition(`selector_exists(xpath("//div[@class='test']"))`)
	require.NoError(t, err)
	ctx := &evalContext{body: `<html><body><div class="test">ok</div></body></html>`}
	result, err := cond.Eval(ctx, nil)
	require.NoError(t, err)
	assert.True(t, result)

	ctx = &evalContext{body: `<html><body></body></html>`}
	result, err = cond.Eval(ctx, nil)
	require.NoError(t, err)
	assert.False(t, result)

	ctx = &evalContext{body: ""}
	result, err = cond.Eval(ctx, nil)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestCompileCondition_Errors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "not_a_call", raw: "not_a_call"},
		{name: "unknown_function", raw: `unknown_fn("a", "b")`},
		{name: "contains_1_arg", raw: `contains("a")`},
		{name: "contains_non_string", raw: `contains(a, b)`},
		{name: "regex_match_bad_pattern", raw: `regex_match("abc", "[invalid")`},
		{name: "regex_match_non_string_first", raw: `regex_match(abc, "pattern")`},
		{name: "regex_match_non_quoted_pattern", raw: `regex_match("abc", pattern)`},
		{name: "regex_match_1_arg", raw: `regex_match("abc")`},
		{name: "selector_exists_0_args", raw: `selector_exists()`},
		{name: "selector_exists_bad_arg", raw: `selector_exists("bad")`},
		{name: "selector_exists_non_xpath", raw: `selector_exists(css("selector"))`},
		{name: "selector_exists_unquoted", raw: `selector_exists(xpath(unquoted))`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileCondition(tt.raw)
			require.Error(t, err)
		})
	}
}

func TestConditionGroupEval_And(t *testing.T) {
	g, err := compileConditionGroup(&ConditionGroupSpec{
		Mode:       "and",
		Conditions: []string{`contains("abc", "a")`, `contains("abc", "b")`},
	})
	require.NoError(t, err)
	ctx := &evalContext{}
	ok, err := g.Eval(ctx, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestConditionGroupEval_Or(t *testing.T) {
	g, err := compileConditionGroup(&ConditionGroupSpec{
		Mode:       "or",
		Conditions: []string{`contains("abc", "x")`, `contains("abc", "b")`},
	})
	require.NoError(t, err)
	ctx := &evalContext{}
	ok, err := g.Eval(ctx, nil)
	require.NoError(t, err)
	assert.True(t, ok)

	g2, err := compileConditionGroup(&ConditionGroupSpec{
		Mode:       "or",
		Conditions: []string{`contains("abc", "x")`, `contains("abc", "y")`},
	})
	require.NoError(t, err)
	ok, err = g2.Eval(ctx, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestConditionGroupEval_Nil(t *testing.T) {
	var g *compiledConditionGroup
	ok, err := g.Eval(&evalContext{}, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

// --- jsonpath tests ---

func TestEvalJSONPathStrings(t *testing.T) {
	doc := map[string]interface{}{
		"str":  "hello",
		"num":  42.0,
		"bool": true,
		"arr":  []interface{}{"a", "b"},
		"nested": map[string]interface{}{
			"key": "val",
		},
	}
	tests := []struct {
		name   string
		expr   string
		expect []string
	}{
		{name: "string", expr: "$.str", expect: []string{"hello"}},
		{name: "number", expr: "$.num", expect: []string{"42"}},
		{name: "bool", expr: "$.bool", expect: []string{"true"}},
		{name: "array", expr: "$.arr[*]", expect: []string{"a", "b"}},
		{name: "missing", expr: "$.missing", expect: nil},
		{name: "nested_object", expr: "$.nested", expect: []string{`{"key":"val"}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evalJSONPathStrings(doc, tt.expr)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestFlattenJSONPathValue_Nil(t *testing.T) {
	var out []string
	flattenJSONPathValue(nil, &out)
	assert.Nil(t, out)
}

func TestIsJSONPathMissingError(t *testing.T) {
	assert.False(t, isJSONPathMissingError(nil))
}

// --- plugin compile/validate tests ---

func TestValidatePluginSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *PluginSpec
		wantErr bool
	}{
		{name: "invalid_version", spec: &PluginSpec{Version: 2}, wantErr: true},
		{name: "empty_name", spec: &PluginSpec{Version: 1}, wantErr: true},
		{name: "invalid_type", spec: &PluginSpec{Version: 1, Name: "test", Type: "bad"}, wantErr: true},
		{name: "no_hosts", spec: &PluginSpec{Version: 1, Name: "test", Type: "one-step"}, wantErr: true},
		{name: "both_request_and_multi", spec: &PluginSpec{
			Version: 1, Name: "test", Type: "one-step", Hosts: []string{"h"},
			Request:      &RequestSpec{Method: "GET", Path: "/"},
			MultiRequest: &MultiRequestSpec{},
		}, wantErr: true},
		{name: "two_step_no_workflow", spec: &PluginSpec{
			Version: 1, Name: "test", Type: "two-step", Hosts: []string{"h"},
			Request: &RequestSpec{Method: "GET", Path: "/"},
		}, wantErr: true},
		{name: "no_request_or_multi", spec: &PluginSpec{
			Version: 1, Name: "test", Type: "one-step", Hosts: []string{"h"},
		}, wantErr: true},
		{name: "invalid_fetch_type", spec: &PluginSpec{
			Version: 1, Name: "test", Type: "one-step", Hosts: []string{"h"},
			FetchType: "bad", Request: &RequestSpec{Method: "GET", Path: "/"},
		}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePluginSpec(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompileRequest_Errors(t *testing.T) {
	tests := []struct {
		name string
		spec *RequestSpec
	}{
		{name: "no_method", spec: &RequestSpec{Path: "/"}},
		{name: "no_path_or_url", spec: &RequestSpec{Method: "GET"}},
		{name: "both_path_and_url", spec: &RequestSpec{Method: "GET", Path: "/", URL: "http://x"}},
		{name: "unsupported_method", spec: &RequestSpec{Method: "DELETE", Path: "/"}},
		{name: "unsupported_body_kind", spec: &RequestSpec{Method: "POST", Path: "/", Body: &RequestBodySpec{Kind: "xml"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileRequest(tt.spec)
			require.Error(t, err)
		})
	}
}

func TestCompileRequest_Nil(t *testing.T) {
	r, err := compileRequest(nil)
	require.NoError(t, err)
	require.Nil(t, r)
}

func TestCompileRequest_Full(t *testing.T) {
	spec := &RequestSpec{
		Method:              "POST",
		Path:                "/api",
		Query:               map[string]string{"q": "val"},
		Headers:             map[string]string{"X-Key": "v"},
		Cookies:             map[string]string{"session": "abc"},
		Body:                &RequestBodySpec{Kind: "json", Values: map[string]string{"a": "b"}},
		Response:            &ResponseSpec{DecodeCharset: "utf-8"},
		Browser:             &BrowserSpec{WaitSelector: "div", WaitTimeout: 10},
		AcceptStatusCodes:   []int{200, 201},
		NotFoundStatusCodes: []int{404},
	}
	r, err := compileRequest(spec)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "POST", r.method)
	assert.NotNil(t, r.body)
	assert.NotNil(t, r.browser)
}

func TestCompileRequestBody(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		wantErr bool
	}{
		{name: "form", kind: "form"},
		{name: "json", kind: "json"},
		{name: "raw", kind: "raw"},
		{name: "invalid", kind: "xml", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileRequestBody(&RequestBodySpec{Kind: tt.kind})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompileRequestBody_WithContent(t *testing.T) {
	b, err := compileRequestBody(&RequestBodySpec{Kind: "raw", Content: "data"})
	require.NoError(t, err)
	require.NotNil(t, b.content)
}

func TestCompileMultiRequest(t *testing.T) {
	tests := []struct {
		name    string
		spec    *MultiRequestSpec
		wantErr bool
	}{
		{name: "no_candidates", spec: &MultiRequestSpec{Request: &RequestSpec{Method: "GET", Path: "/"}}, wantErr: true},
		{name: "no_request", spec: &MultiRequestSpec{Candidates: []string{"a"}}, wantErr: true},
		{name: "valid", spec: &MultiRequestSpec{
			Candidates: []string{"a"},
			Request:    &RequestSpec{Method: "GET", Path: "/"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileMultiRequest(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompileScrape(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ScrapeSpec
		wantErr bool
	}{
		{name: "nil", spec: nil, wantErr: true},
		{name: "bad_format", spec: &ScrapeSpec{Format: "xml"}, wantErr: true},
		{name: "no_fields", spec: &ScrapeSpec{Format: "html"}, wantErr: true},
		{name: "field_no_selector", spec: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {},
		}}, wantErr: true},
		{name: "html_non_xpath", spec: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "jsonpath", Expr: "$.title"}},
		}}, wantErr: true},
		{name: "json_non_jsonpath", spec: &ScrapeSpec{Format: "json", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title"}},
		}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileScrape(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompilePostprocess(t *testing.T) {
	p, err := compilePostprocess(nil)
	require.NoError(t, err)
	require.Nil(t, p)

	p, err = compilePostprocess(&PostprocessSpec{
		Assign: map[string]string{"number": "${number}"},
	})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestCompilePrecheck(t *testing.T) {
	p, err := compilePrecheck(nil)
	require.NoError(t, err)
	require.Nil(t, p)

	p, err = compilePrecheck(&PrecheckSpec{
		NumberPatterns: []string{`^ABC`},
		Variables:      map[string]string{"x": "${number}"},
	})
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Len(t, p.numberPatterns, 1)
}

// --- applyStringTransforms ---

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

func TestNormalizeLang(t *testing.T) {
	assert.Equal(t, "", normalizeLang(""))
	assert.NotEmpty(t, normalizeLang("ja"))
	assert.NotEmpty(t, normalizeLang("en"))
	assert.NotEmpty(t, normalizeLang("zh-cn"))
	assert.NotEmpty(t, normalizeLang("zh-tw"))
	assert.Equal(t, "custom", normalizeLang("custom"))
}

// --- checkAcceptedStatus ---

func TestCheckAcceptedStatus(t *testing.T) {
	tests := []struct {
		name    string
		spec    *compiledRequest
		code    int
		wantErr bool
	}{
		{name: "200_default", spec: &compiledRequest{}, code: 200},
		{name: "404_default", spec: &compiledRequest{}, code: 404, wantErr: true},
		{name: "in_accept_list", spec: &compiledRequest{acceptStatusCodes: []int{200, 201}}, code: 201},
		{name: "not_in_accept_list", spec: &compiledRequest{acceptStatusCodes: []int{200}}, code: 404, wantErr: true},
		{name: "in_not_found_list", spec: &compiledRequest{notFoundStatusCodes: []int{404}}, code: 404, wantErr: true},
		{name: "non200_no_accept_list", spec: &compiledRequest{}, code: 500, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAcceptedStatus(tt.spec, tt.code)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- decodeBytes ---

func TestDecodeBytes(t *testing.T) {
	data := []byte("hello")
	result, err := decodeBytes(data, "")
	require.NoError(t, err)
	assert.Equal(t, data, result)

	result, err = decodeBytes(data, "utf-8")
	require.NoError(t, err)
	assert.Equal(t, data, result)

	result, err = decodeBytes(data, "utf8")
	require.NoError(t, err)
	assert.Equal(t, data, result)

	eucjpData := []byte{0xa4, 0xa2}
	result, err = decodeBytes(eucjpData, "euc-jp")
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	_, err = decodeBytes(data, "unknown-charset")
	require.Error(t, err)
}

// --- buildURL ---

func TestBuildURL(t *testing.T) {
	assert.Equal(t, "https://example.com/path", buildURL("https://example.com", "/path"))
	assert.Contains(t, buildURL("://bad", "/path"), "/path")
}

// --- timeParse / softTimeParse ---

func TestTimeParse(t *testing.T) {
	_, err := timeParse("2006-01-02", "2024-01-02")
	require.NoError(t, err)

	_, err = timeParse("2006-01-02", "bad")
	require.Error(t, err)
}

func TestSoftTimeParse(t *testing.T) {
	assert.NotZero(t, softTimeParse("2006-01-02", "2024-01-02"))
	assert.Zero(t, softTimeParse("2006-01-02", "bad"))
}

// --- regexpMatch ---

func TestRegexpMatch(t *testing.T) {
	ok, err := regexpMatch(`^ABC`, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = regexpMatch(`^XYZ`, "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = regexpMatch(`[invalid`, "ABC")
	require.Error(t, err)
}

// --- spec UnmarshalJSON ---

func TestParserSpecUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		kind   string
		layout string
	}{
		{name: "string", input: `"string"`, kind: "string"},
		{name: "object", input: `{"kind":"time_format","layout":"2006-01-02"}`, kind: "time_format", layout: "2006-01-02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p ParserSpec
			err := json.Unmarshal([]byte(tt.input), &p)
			require.NoError(t, err)
			assert.Equal(t, tt.kind, p.Kind)
			assert.Equal(t, tt.layout, p.Layout)
		})
	}
}

func TestParserSpecUnmarshalJSON_Error(t *testing.T) {
	var p ParserSpec
	err := json.Unmarshal([]byte(`{"kind": 123}`), &p)
	require.Error(t, err)

	err = json.Unmarshal([]byte(`"unterminated`), &p)
	require.Error(t, err)
}

func TestParserSpecUnmarshalYAML_Error(t *testing.T) {
	raw := `parser: [1, 2]`
	type wrapper struct {
		Parser ParserSpec `yaml:"parser"`
	}
	var w wrapper
	err := yaml.Unmarshal([]byte(raw), &w)
	require.Error(t, err)
}

// --- OnPrecheckRequest ---

func TestOnPrecheckRequest(t *testing.T) {
	plgYAML := `
version: 1
name: test
type: one-step
hosts:
  - https://example.com
precheck:
  number_patterns:
    - "^ABC"
  variables:
    clean_num: ${clean_number(${number})}
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
`
	plg := mustCompilePlugin(t, plgYAML)
	ctx := pluginapi.InitContainer(context.Background())

	ok, err := plg.OnPrecheckRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = plg.OnPrecheckRequest(ctx, "XYZ-123")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnPrecheckRequest_NoPrecheck(t *testing.T) {
	plgYAML := `
version: 1
name: test
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
`
	plg := mustCompilePlugin(t, plgYAML)
	ctx := pluginapi.InitContainer(context.Background())
	ok, err := plg.OnPrecheckRequest(ctx, "anything")
	require.NoError(t, err)
	assert.True(t, ok)
}

// --- OnPrecheckResponse ---

func TestOnPrecheckResponse(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)

	tests := []struct {
		name   string
		code   int
		expect bool
	}{
		{name: "200", code: 200, expect: true},
		{name: "404", code: 404, expect: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsp := &http.Response{StatusCode: tt.code}
			ok, err := plg.OnPrecheckResponse(context.Background(), req, rsp)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, ok)
		})
	}
}

// --- OnDecorateMediaRequest ---

func TestOnDecorateMediaRequest(t *testing.T) {
	plgYAML := `
version: 1
name: test
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/${number}
  headers:
    X-Custom: header-val
  cookies:
    session: cookie-val
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
`
	plg := mustCompilePlugin(t, plgYAML)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	pluginapi.SetContainerValue(ctx, ctxKeyFinalPage, "https://example.com/page")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/img.jpg", nil)
	err := plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "header-val", req.Header.Get("X-Custom"))
	assert.Equal(t, "https://example.com/page", req.Header.Get("Referer"))
}

// --- buildRequestBodyReader ---

func TestBuildRequestBodyReader(t *testing.T) {
	ctx := &evalContext{number: "ABC-123"}

	jsonBody, _ := compileRequestBody(&RequestBodySpec{Kind: "json", Values: map[string]string{"a": "b"}})
	r, err := buildRequestBodyReader(&compiledRequest{body: jsonBody}, ctx)
	require.NoError(t, err)
	require.NotNil(t, r)

	rawBody, _ := compileRequestBody(&RequestBodySpec{Kind: "raw", Content: "raw data"})
	r, err = buildRequestBodyReader(&compiledRequest{body: rawBody}, ctx)
	require.NoError(t, err)
	require.NotNil(t, r)

	r, err = buildRequestBodyReader(&compiledRequest{body: nil}, ctx)
	require.NoError(t, err)
	require.Nil(t, r)

	rawNoContent, _ := compileRequestBody(&RequestBodySpec{Kind: "raw"})
	r, err = buildRequestBodyReader(&compiledRequest{body: rawNoContent}, ctx)
	require.NoError(t, err)
	require.Nil(t, r)
}

// --- previewBody ---

func TestPreviewBody(t *testing.T) {
	short := "hello"
	assert.Equal(t, short, previewBody(short))

	long := strings.Repeat("a", 5000)
	assert.Len(t, previewBody(long), 4000)
}

// --- equalNormalizedSet / normalizeStringSet ---

func TestEqualNormalizedSet(t *testing.T) {
	assert.True(t, equalNormalizedSet([]string{"a", "b"}, []string{"b", "a"}))
	assert.False(t, equalNormalizedSet([]string{"a"}, []string{"b"}))
	assert.False(t, equalNormalizedSet([]string{"a", "b"}, []string{"a"}))
	assert.True(t, equalNormalizedSet([]string{"a", "a"}, []string{"a"}))
}

func TestNormalizeStringSet(t *testing.T) {
	result := normalizeStringSet([]string{"b", " a ", "", "b"})
	assert.Equal(t, []string{"a", "b"}, result)
}

// --- renderCondition ---

func TestRenderCondition(t *testing.T) {
	assert.Equal(t, "", renderCondition(nil))
	assert.Equal(t, "contains", renderCondition(&compiledCondition{name: "contains"}))
}

// --- isListField ---

func TestIsListField(t *testing.T) {
	assert.True(t, isListField("actors"))
	assert.True(t, isListField("genres"))
	assert.True(t, isListField("sample_images"))
	assert.False(t, isListField("title"))
}

// --- movieMetaStringMap ---

func TestMovieMetaStringMap(t *testing.T) {
	mv := &model.MovieMeta{
		Number: "N", Title: "T", Cover: &model.File{Name: "C"}, Poster: &model.File{Name: "P"},
	}
	m := movieMetaStringMap(mv)
	assert.Equal(t, "N", m["number"])
	assert.Equal(t, "C", m["cover"])
	assert.Equal(t, "P", m["poster"])

	mv2 := &model.MovieMeta{Number: "N"}
	m2 := movieMetaStringMap(mv2)
	assert.Empty(t, m2["cover"])
}

// --- readVarsFromContext ---

func TestReadVarsFromContext(t *testing.T) {
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, "yaml.var.x", "v1")
	pluginapi.SetContainerValue(ctx, "other.key", "v2")
	vars := readVarsFromContext(ctx)
	assert.Equal(t, "v1", vars["x"])
	assert.NotContains(t, vars, "other.key")
}

// --- ctxVarKey ---

func TestCtxVarKey(t *testing.T) {
	assert.Equal(t, "yaml.var.myvar", ctxVarKey("myvar"))
}

// --- cachedCreator ---

func TestCachedCreator_Error(t *testing.T) {
	cc := &cachedCreator{data: []byte("invalid yaml")}
	_, err := cc.create(nil)
	require.Error(t, err)

	_, err = cc.create(nil)
	require.Error(t, err)
}

func TestCachedCreator_Success(t *testing.T) {
	cc := &cachedCreator{data: []byte(minimalOneStepYAML())}
	plg1, err := cc.create(nil)
	require.NoError(t, err)
	require.NotNil(t, plg1)

	plg2, err := cc.create(nil)
	require.NoError(t, err)
	require.Equal(t, plg1, plg2)
}

// --- currentHost ---

func TestCurrentHost(t *testing.T) {
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://cached.com")
	assert.Equal(t, "https://cached.com", currentHost(ctx, []string{"https://other.com"}))

	ctx2 := pluginapi.InitContainer(context.Background())
	host := currentHost(ctx2, []string{"https://only.com"})
	assert.Equal(t, "https://only.com", host)
}

// --- assignDateField ---

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

func TestCheckBaseResponseStatus(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	err := checkBaseResponseStatus(plg, 200)
	require.NoError(t, err)

	err = checkBaseResponseStatus(plg, 500)
	require.Error(t, err)
}

// --- NewFromBytes ---

func TestNewFromBytes_InvalidYAML(t *testing.T) {
	_, err := NewFromBytes([]byte(":::invalid"))
	require.Error(t, err)
}

func TestNewFromBytes_Valid(t *testing.T) {
	plg, err := NewFromBytes([]byte(minimalOneStepYAML()))
	require.NoError(t, err)
	require.NotNil(t, plg)
}

// --- OnGetHosts ---

func TestOnGetHosts(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	hosts := plg.OnGetHosts(context.Background())
	assert.Equal(t, []string{"https://example.com"}, hosts)
}

// --- OnDecorateRequest (no-op) ---

func TestOnDecorateRequest(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	err := plg.OnDecorateRequest(context.Background(), req)
	require.NoError(t, err)
}

// --- setBodyContentType ---

func TestSetBodyContentType(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
	setBodyContentType(req, &compiledRequest{body: nil})
	assert.Empty(t, req.Header.Get("Content-Type"))

	formBody, _ := compileRequestBody(&RequestBodySpec{Kind: "form"})
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
	setBodyContentType(req2, &compiledRequest{body: formBody})
	assert.Equal(t, "application/x-www-form-urlencoded", req2.Header.Get("Content-Type"))

	jsonBody, _ := compileRequestBody(&RequestBodySpec{Kind: "json"})
	req3, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
	setBodyContentType(req3, &compiledRequest{body: jsonBody})
	assert.Equal(t, "application/json", req3.Header.Get("Content-Type"))

	req4, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com/", nil)
	req4.Header.Set("Content-Type", "custom")
	setBodyContentType(req4, &compiledRequest{body: formBody})
	assert.Equal(t, "custom", req4.Header.Get("Content-Type"))
}

// --- CompileDraft ---

func TestCompileDraft(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	result, err := CompileDraft(spec)
	require.NoError(t, err)
	require.Contains(t, result.YAML, "version: 1")
	assert.True(t, result.Summary.HasRequest)
}

func TestCompileDraft_Error(t *testing.T) {
	_, err := CompileDraft(&PluginSpec{Version: 0})
	require.Error(t, err)
}

// --- helpers ---

func mustCompilePlugin(t *testing.T, yamlStr string) *SearchPlugin {
	t.Helper()
	plg, err := NewFromBytes([]byte(yamlStr))
	require.NoError(t, err)
	return plg.(*SearchPlugin)
}

func minimalOneStepYAML() string {
	return `
version: 1
name: test
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
`
}

// --- NEW: compileWorkflow / compileSearchSelect ---

func TestCompileWorkflow_Exhaustive(t *testing.T) {
	_, err := compileWorkflow(nil)
	assert.NoError(t, err)

	_, err = compileWorkflow(&WorkflowSpec{})
	assert.NoError(t, err)

	_, err = compileWorkflow(&WorkflowSpec{SearchSelect: &SearchSelectWorkflowSpec{
		Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a/@href"}},
		Return:      "${item.link}",
		NextRequest: &RequestSpec{Method: "GET", Path: "/${value}"},
	}})
	assert.NoError(t, err)
}

func TestCompileSearchSelect_Exhaustive(t *testing.T) {
	_, err := compileSearchSelect(&SearchSelectWorkflowSpec{})
	assert.ErrorIs(t, err, errSearchSelectRequiresSelector)

	_, err = compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors: []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
	})
	assert.ErrorIs(t, err, errSearchSelectNextRequestRequired)

	_, err = compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors:   []*SelectorListSpec{{Name: "link", Kind: "jsonpath", Expr: "$.a"}},
		NextRequest: &RequestSpec{Method: "GET", Path: "/${value}"},
	})
	assert.ErrorIs(t, err, errUnsupportedSelectorKind)

	result, err := compileSearchSelect(&SearchSelectWorkflowSpec{
		Selectors:     []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a"}},
		Return:        "${item.link}",
		NextRequest:   &RequestSpec{Method: "GET", Path: "/${value}"},
		ItemVariables: map[string]string{"slug": "${item.link}"},
		Match:         &ConditionGroupSpec{Mode: "and", Conditions: []string{`contains("${item.link}", "video")`}},
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.itemVariables, 1)
}

// --- NEW: OnPrecheckRequest_WithPattern ---

func TestOnPrecheckRequest_WithPattern(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
precheck:
  number_patterns: ["^[A-Z]+-\\d+$"]
  variables:
    slug: "${to_lower(${number})}"
request:
  method: GET
  path: /search/${vars.slug}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())

	ok, err := plg.OnPrecheckRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = plg.OnPrecheckRequest(ctx, "abc123")
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- OnMakeHTTPRequest ---

func TestOnMakeHTTPRequest(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.Contains(t, req.URL.String(), "/search/ABC-123")
}

func TestOnMakeHTTPRequest_MultiRequest(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	require.NotNil(t, req)
}

func TestOnPrecheckResponse_AcceptStatusCodes(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  accept_status_codes: [200, 301]
  not_found_status_codes: [404]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)

	ok, err := plg.OnPrecheckResponse(ctx, req, &http.Response{StatusCode: 200})
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, _ = plg.OnPrecheckResponse(ctx, req, &http.Response{StatusCode: 404})
	assert.False(t, ok)

	_, err = plg.OnPrecheckResponse(ctx, req, &http.Response{StatusCode: 500})
	assert.Error(t, err)
}

// --- OnDecodeHTTPData ---

func TestOnDecodeHTTPData_HTML(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`<html><head><title>MyTitle</title></head></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "MyTitle", mv.Title)
}

func TestOnDecodeHTTPData_JSON(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /api/${number}
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: "$.title"
    actors:
      selector:
        kind: jsonpath
        expr: "$.actors[*]"
      parser: string_list
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`{"title":"T","actors":["A","B"]}`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "T", mv.Title)
	assert.Equal(t, []string{"A", "B"}, mv.Actors)
}

func TestOnDecodeHTTPData_NotFound(t *testing.T) {
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
        expr: //h1[@class="title"]/text()
      required: true
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	data := []byte(`<html><body><h1>no-class</h1></body></html>`)
	_, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- applyPostprocess ---

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

func TestBuildRequest_FormBody(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "POST",
			Path:   "/api",
			Body:   &RequestBodySpec{Kind: "form", Values: map[string]string{"q": "${number}"}},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, compiled.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
}

func TestBuildRequest_JSONBody(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "POST",
			Path:   "/api",
			Body:   &RequestBodySpec{Kind: "json", Values: map[string]string{"q": "${number}"}},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, compiled.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestBuildRequest_RawBody(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "POST",
			Path:   "/api",
			Body:   &RequestBodySpec{Kind: "raw", Content: "raw-data-${number}"},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, compiled.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.NotNil(t, req.Body)
}

func TestBuildRequest_WithURL(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "test",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{
			Method: "GET",
			URL:    "https://other.com/search/${number}",
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}}}},
	}
	compiled, err := compilePlugin(spec)
	require.NoError(t, err)
	plg := &SearchPlugin{spec: compiled}
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, compiled.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "https://other.com/search/ABC", req.URL.String())
}

// --- applyRequestParams ---

func TestApplyRequestParams(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	queryTmpl, _ := compileTemplate("val")
	headerTmpl, _ := compileTemplate("hval")
	cookieTmpl, _ := compileTemplate("cval")
	spec := &compiledRequest{
		query:   map[string]*template{"q": queryTmpl},
		headers: map[string]*template{"X-H": headerTmpl},
		cookies: map[string]*template{"c": cookieTmpl},
	}
	err := applyRequestParams(req, spec, &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "val", req.URL.Query().Get("q"))
	assert.Equal(t, "hval", req.Header.Get("X-H"))
	found := false
	for _, c := range req.Cookies() {
		if c.Name == "c" && c.Value == "cval" {
			found = true
		}
	}
	assert.True(t, found)
}

// --- applyFetchTypeContext ---

func TestApplyFetchTypeContext_GoHTTP(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	result := plg.applyFetchTypeContext(req, plg.spec.request)
	assert.Equal(t, req, result)
}

func TestApplyFetchTypeContext_Flaresolverr(t *testing.T) {
	yamlStr := `
version: 1
name: test-flare
type: one-step
fetch_type: flaresolverr
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
`
	plg := mustCompilePlugin(t, yamlStr)
	assert.Equal(t, fetchTypeFlaresolverr, plg.spec.fetchType)
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.NotNil(t, req)

	bp := browser.GetParams(req.Context())
	assert.Nil(t, bp, "browser params should not be set for flaresolverr")

	fp := flarerr.GetParams(req.Context())
	assert.NotNil(t, fp, "flarerr params should be set for flaresolverr fetch_type")
}

func TestValidate_FlaresolverrFetchType(t *testing.T) {
	yamlStr := `
version: 1
name: test-flare
type: one-step
fetch_type: flaresolverr
hosts: ["https://example.com"]
request:
  method: GET
  path: /x
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	assert.Equal(t, fetchTypeFlaresolverr, plg.spec.fetchType)
}

func TestValidate_InvalidFetchType(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: unknown
hosts: ["https://example.com"]
request:
  method: GET
  path: /x
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	_, err := NewFromBytes([]byte(yamlStr))
	require.Error(t, err)
	assert.ErrorIs(t, err, errInvalidFetchType)
}

func TestApplyBrowserContext_BrowserFetchType(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: browser
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  browser:
    wait_selector: ".content"
    wait_timeout: 5
  headers:
    X-Custom: "val"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.NotNil(t, req)
	params := browser.GetParams(req.Context())
	require.NotNil(t, params)
	assert.Equal(t, ".content", params.WaitSelector)
	assert.Equal(t, 5*time.Second, params.WaitTimeout)
	assert.Equal(t, time.Duration(0), params.WaitStableDuration)
}

func TestApplyBrowserContext_WaitStable_Explicit(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: browser
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  browser:
    wait_stable: 10
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	params := browser.GetParams(req.Context())
	require.NotNil(t, params)
	assert.Equal(t, 10*time.Second, params.WaitStableDuration)
}

func TestApplyBrowserContext_WaitStable_DefaultWhenNoSelector(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: browser
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
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	params := browser.GetParams(req.Context())
	require.NotNil(t, params)
	assert.Equal(t, "", params.WaitSelector)
	assert.Equal(t, defaultWaitStable, params.WaitStableDuration)
}

func TestApplyBrowserContext_WaitStable_NotAppliedWithSelector(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
fetch_type: browser
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  browser:
    wait_selector: "//div"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	req, err := plg.buildRequest(ctx, plg.spec.request, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	params := browser.GetParams(req.Context())
	require.NotNil(t, params)
	assert.Equal(t, "//div", params.WaitSelector)
	assert.Equal(t, time.Duration(0), params.WaitStableDuration)
}

// --- template edge cases ---

func TestTemplateValidateArg_NestedCalls(t *testing.T) {
	err := validateTemplateArg(`to_upper("abc")`)
	assert.NoError(t, err)

	err = validateTemplateArg(`${number}`)
	assert.NoError(t, err)

	err = validateTemplateArg(`abc${number}def`)
	assert.NoError(t, err)

	err = validateTemplateArg(`"literal"`)
	assert.NoError(t, err)

	err = validateTemplateArg(``)
	assert.NoError(t, err)

	err = validateTemplateArg(`number`)
	assert.NoError(t, err)
}

func TestEvalTemplateArg_Branches(t *testing.T) {
	ctx := &evalContext{number: "ABC"}

	v, err := evalTemplateArg(`"literal"`, ctx)
	require.NoError(t, err)
	assert.Equal(t, "literal", v)

	v, err = evalTemplateArg(`${number}`, ctx)
	require.NoError(t, err)
	assert.Equal(t, "ABC", v)

	v, err = evalTemplateArg(`prefix${number}suffix`, ctx)
	require.NoError(t, err)
	assert.Equal(t, "prefixABCsuffix", v)

	v, err = evalTemplateArg(`to_upper("abc")`, ctx)
	require.NoError(t, err)
	assert.Equal(t, "ABC", v)

	v, err = evalTemplateArg(`number`, ctx)
	require.NoError(t, err)
	assert.Equal(t, "ABC", v)

	v, err = evalTemplateArg(`plaintext`, ctx)
	require.NoError(t, err)
	assert.Equal(t, "plaintext", v)
}

// --- condition edge cases ---

func TestConditionGroup_OrMode(t *testing.T) {
	spec := &ConditionGroupSpec{
		Mode:       "or",
		Conditions: []string{`equals("a", "b")`, `equals("c", "c")`},
	}
	group, err := compileConditionGroup(spec)
	require.NoError(t, err)
	ok, err := group.Eval(&evalContext{}, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestConditionGroup_OrMode_AllFalse(t *testing.T) {
	spec := &ConditionGroupSpec{
		Mode:       "or",
		Conditions: []string{`equals("a", "b")`, `equals("c", "d")`},
	}
	group, err := compileConditionGroup(spec)
	require.NoError(t, err)
	ok, err := group.Eval(&evalContext{}, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestConditionErrors(t *testing.T) {
	_, err := compileConditionGroup(&ConditionGroupSpec{Mode: "bad"})
	assert.Error(t, err)

	_, err = compileConditionGroup(&ConditionGroupSpec{Mode: "and"})
	assert.Error(t, err)

	_, err = compileCondition("not_a_call")
	assert.Error(t, err)

	_, err = compileCondition(`unknown_func("a", "b")`)
	assert.Error(t, err)

	_, err = compileCondition(`contains("a")`)
	assert.Error(t, err)

	_, err = compileCondition(`regex_match("a")`)
	assert.Error(t, err)

	_, err = compileCondition(`regex_match("a", unquoted)`)
	assert.Error(t, err)

	_, err = compileCondition(`selector_exists("bad")`)
	assert.Error(t, err)

	_, err = compileCondition(`selector_exists(xpath("a"), xpath("b"))`)
	assert.Error(t, err)

	_, err = compileCondition(`contains("a", unquoted)`)
	assert.Error(t, err)
}

func TestEvalSelectorExists_WithBody(t *testing.T) {
	sel, err := compileSelectorLiteral(`xpath("//div")`)
	require.NoError(t, err)
	args := []compiledConditionArg{{kind: "selector", selector: sel}}
	ctx := &evalContext{body: `<html><body><div>found</div></body></html>`}
	ok, err := evalSelectorExists(args, ctx, nil)
	require.NoError(t, err)
	assert.True(t, ok)

	ctx2 := &evalContext{body: ""}
	ok, err = evalSelectorExists(args, ctx2, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- splitArgs edge cases ---

func TestSplitArgs_Errors(t *testing.T) {
	_, err := splitArgs(`"unterminated`)
	assert.Error(t, err)

	_, err = splitArgs(`(unbalanced`)
	assert.Error(t, err)

	_, err = splitArgs(`)extra_close`)
	assert.Error(t, err)
}

// --- movieMetaStringMap ---

func TestMovieMetaStringMap_NilFields(t *testing.T) {
	mv := &model.MovieMeta{Number: "N", Title: "T"}
	m := movieMetaStringMap(mv)
	assert.Equal(t, "N", m["number"])
	assert.Equal(t, "T", m["title"])
	assert.Empty(t, m["cover"])
	assert.Empty(t, m["poster"])
}

// --- finalRequest ---

func TestFinalRequest(t *testing.T) {
	reqSpec := &compiledRequest{method: "GET"}

	p1 := &compiledPlugin{request: reqSpec}
	assert.Equal(t, reqSpec, p1.finalRequest())

	nextReq := &compiledRequest{method: "POST"}
	p2 := &compiledPlugin{workflow: &compiledSearchSelectWorkflow{nextRequest: nextReq}}
	assert.Equal(t, nextReq, p2.finalRequest())

	mrReq := &compiledRequest{method: "PUT"}
	p3 := &compiledPlugin{multiRequest: &compiledMultiRequest{request: mrReq}}
	assert.Equal(t, mrReq, p3.finalRequest())
}

// --- jsonpath edge cases ---

func TestEvalJSONPathStrings_Missing(t *testing.T) {
	doc := map[string]any{"a": "b"}
	values, err := evalJSONPathStrings(doc, "$.missing")
	assert.NoError(t, err)
	assert.Nil(t, values)
}

func TestFlattenJSONPathValue_Types(t *testing.T) {
	var out []string
	flattenJSONPathValue(nil, &out)
	assert.Empty(t, out)

	flattenJSONPathValue(float64(3.14), &out)
	assert.Len(t, out, 1)

	flattenJSONPathValue(true, &out)
	assert.Len(t, out, 2)

	flattenJSONPathValue(map[string]any{"k": "v"}, &out)
	assert.Len(t, out, 3)
}

// --- OnHandleHTTPRequest with multi_request ---

func TestOnHandleHTTPRequest_MultiRequest_Match(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}", "${to_lower(${number})}"]
  unique: true
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div[@class=''found'']"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		body := `<html><body><div class="found">ok</div></body></html>`
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(body)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
	assert.Equal(t, 200, rsp.StatusCode)
}

func TestOnHandleHTTPRequest_MultiRequest_NoMatch(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div[@class=''found'']"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body>no match</body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

func TestOnHandleHTTPRequest_MultiRequest_StatusRejected(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 404,
			Body:       nopCloser([]byte("not found")),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- OnHandleHTTPRequest with workflow ---

func TestOnHandleHTTPRequest_Workflow(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><a href="/detail/1">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><h1>Detail</h1></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
}

func TestOnHandleHTTPRequest_PlainRequest(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><title>T</title></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
}

// --- OnHandleHTTPRequest with multi_request + workflow ---

func TestOnHandleHTTPRequest_MultiRequest_Workflow(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div[@class=''found'']"))'
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><div class="found">ok</div><a href="/detail/1">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><h1>Detail</h1></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
}

// --- checkBaseResponseStatus ---

func TestCheckBaseResponseStatus_WithMultiRequest(t *testing.T) {
	reqSpec := &compiledRequest{acceptStatusCodes: []int{200}}
	plg := &SearchPlugin{spec: &compiledPlugin{
		multiRequest: &compiledMultiRequest{request: reqSpec},
	}}
	assert.NoError(t, checkBaseResponseStatus(plg, 200))
	assert.Error(t, checkBaseResponseStatus(plg, 500))
}

func TestCheckBaseResponseStatus_NoRequest(t *testing.T) {
	plg := &SearchPlugin{spec: &compiledPlugin{}}
	assert.NoError(t, checkBaseResponseStatus(plg, 200))
	assert.NoError(t, checkBaseResponseStatus(plg, 500))
}

// --- Workflow with match conditions ---

func TestOnHandleHTTPRequest_WorkflowWithMatch(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
      - name: txt
        kind: xpath
        expr: "//a/text()"
    item_variables:
      slug: "${item.link}"
    match:
      mode: and
      conditions:
        - 'contains("${item.txt}", "video")'
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body: nopCloser([]byte(`<html><body>
					<a href="/detail/1">image</a>
					<a href="/detail/2">video page</a>
				</body></html>`)),
				Header: make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><h1>Detail</h1></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	require.NotNil(t, rsp)
	defer func() { _ = rsp.Body.Close() }()
}

// --- Workflow with expect_count ---

func TestOnHandleHTTPRequest_WorkflowExpectCountMismatch(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    match:
      mode: and
      conditions:
        - 'contains("${item.link}", "video")'
      expect_count: 5
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><a href="/video/1">video</a></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- OnDecodeHTTPData with JSON required not found ---

func TestOnDecodeHTTPData_JSON_RequiredNotFound(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /api/${number}
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: "$.title"
      required: true
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`{"other":"value"}`)
	_, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestOnDecodeHTTPData_JSON_WithListFields(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /api/${number}
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: "$.title"
    genres:
      selector:
        kind: jsonpath
        expr: "$.genres[*]"
      parser: string_list
    sample_images:
      selector:
        kind: jsonpath
        expr: "$.images[*]"
      parser: string_list
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`{"title":"T","genres":["Action","Drama"],"images":["img1.jpg"]}`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"Action", "Drama"}, mv.Genres)
	assert.Len(t, mv.SampleImages, 1)
}

// --- compilePlugin with full workflow + multi_request ---

func TestCompilePlugin_FullTwoStep(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "full",
		Type:    "two-step",
		Hosts:   []string{"https://example.com"},
		Precheck: &PrecheckSpec{
			NumberPatterns: []string{`.*`},
			Variables:      map[string]string{"slug": "${number}"},
		},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Workflow: &WorkflowSpec{SearchSelect: &SearchSelectWorkflowSpec{
			Selectors:   []*SelectorListSpec{{Name: "link", Kind: "xpath", Expr: "//a/@href"}},
			Return:      "${item.link}",
			NextRequest: &RequestSpec{Method: "GET", Path: "/${value}"},
		}},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}},
		}},
		Postprocess: &PostprocessSpec{Assign: map[string]string{"title": "${meta.title}"}},
	}
	_, err := compilePlugin(spec)
	assert.NoError(t, err)
}

func TestCompilePlugin_WithMultiRequest(t *testing.T) {
	spec := &PluginSpec{
		Version: 1,
		Name:    "mr",
		Type:    "one-step",
		Hosts:   []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Candidates:  []string{"${number}"},
			Request:     &RequestSpec{Method: "GET", Path: "/search/${candidate}"},
			SuccessWhen: &ConditionGroupSpec{Mode: "and", Conditions: []string{`selector_exists(xpath("//div"))`}},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{
			"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//t/text()"}},
		}},
	}
	_, err := compilePlugin(spec)
	assert.NoError(t, err)
}

// --- OnDecodeHTTPData with HTML list fields ---

func TestOnDecodeHTTPData_HTML_AllFields(t *testing.T) {
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
        expr: //h1/text()
    actors:
      selector:
        kind: xpath
        expr: "//div[@class='actors']/span/text()"
      parser: string_list
    genres:
      selector:
        kind: xpath
        expr: "//div[@class='genres']/span/text()"
      parser: string_list
      required: true
    sample_images:
      selector:
        kind: xpath
        expr: "//div[@class='images']/img/@src"
      parser: string_list
    cover:
      selector:
        kind: xpath
        expr: "//img[@class='cover']/@src"
    poster:
      selector:
        kind: xpath
        expr: "//img[@class='poster']/@src"
    number:
      selector:
        kind: xpath
        expr: "//span[@class='num']/text()"
    plot:
      selector:
        kind: xpath
        expr: "//div[@class='plot']/text()"
    studio:
      selector:
        kind: xpath
        expr: "//span[@class='studio']/text()"
    label:
      selector:
        kind: xpath
        expr: "//span[@class='label']/text()"
    director:
      selector:
        kind: xpath
        expr: "//span[@class='director']/text()"
    series:
      selector:
        kind: xpath
        expr: "//span[@class='series']/text()"
    release_date:
      selector:
        kind: xpath
        expr: "//span[@class='date']/text()"
      parser: date_only
    duration:
      selector:
        kind: xpath
        expr: "//span[@class='dur']/text()"
      parser: duration_default
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`<html><body>
		<h1>Title</h1>
		<div class="actors"><span>A</span><span>B</span></div>
		<div class="genres"><span>Action</span></div>
		<div class="images"><img src="img1.jpg"/></div>
		<img class="cover" src="cover.jpg"/>
		<img class="poster" src="poster.jpg"/>
		<span class="num">NUM-1</span>
		<div class="plot">Plot text</div>
		<span class="studio">Studio</span>
		<span class="label">Label</span>
		<span class="director">Dir</span>
		<span class="series">Series</span>
		<span class="date">2024-01-15</span>
		<span class="dur">120分</span>
	</body></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Title", mv.Title)
	assert.Equal(t, []string{"A", "B"}, mv.Actors)
	assert.Equal(t, []string{"Action"}, mv.Genres)
	assert.Len(t, mv.SampleImages, 1)
	assert.Equal(t, "cover.jpg", mv.Cover.Name)
	assert.Equal(t, "poster.jpg", mv.Poster.Name)
}

// --- nopCloser helper ---

func nopCloser(data []byte) io.ReadCloser {
	return io.NopCloser(strings.NewReader(string(data)))
}

// --- Condition eval edge: starts_with, ends_with ---

func TestConditionEval_StartsWithEndsWith(t *testing.T) {
	cond, err := compileCondition(`starts_with("${number}", "ABC")`)
	require.NoError(t, err)
	ok, err := cond.Eval(&evalContext{number: "ABC-123"}, nil)
	require.NoError(t, err)
	assert.True(t, ok)

	cond2, err := compileCondition(`ends_with("${number}", "123")`)
	require.NoError(t, err)
	ok, err = cond2.Eval(&evalContext{number: "ABC-123"}, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

// --- readResponseBody ---

func TestReadResponseBody(t *testing.T) {
	rsp := &http.Response{
		StatusCode: 200,
		Body:       nopCloser([]byte(`<html><body>hi</body></html>`)),
		Header:     make(http.Header),
	}
	body, node, err := readResponseBody(rsp, "")
	require.NoError(t, err)
	assert.Contains(t, body, "hi")
	assert.NotNil(t, node)
}

// --- collectSelectorResults ---

func TestCollectSelectorResults(t *testing.T) {
	htmlStr := `<html><body><a href="1">A</a><a href="2">B</a></body></html>`
	node, _ := parseHTML(htmlStr)
	selectors := []*compiledSelectorList{
		{name: "link", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/@href"}},
		{name: "text", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/text()"}},
	}
	results, count, err := collectSelectorResults(node, selectors)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []string{"1", "2"}, results["link"])
	assert.Equal(t, []string{"A", "B"}, results["text"])
}

func TestCollectSelectorResults_Mismatch(t *testing.T) {
	htmlStr := `<html><body><a href="1">A</a><a href="2">B</a><span>only-one</span></body></html>`
	node, _ := parseHTML(htmlStr)
	selectors := []*compiledSelectorList{
		{name: "link", compiledSelector: compiledSelector{kind: "xpath", expr: "//a/@href"}},
		{name: "span", compiledSelector: compiledSelector{kind: "xpath", expr: "//span/text()"}},
	}
	_, _, err := collectSelectorResults(node, selectors)
	assert.ErrorIs(t, err, errSelectorCountMismatch)
}

func parseHTML(s string) (*html.Node, error) {
	return html.Parse(strings.NewReader(s))
}

// --- OnDecorateMediaRequest with referer ---

func TestOnDecorateMediaRequest_WithReferer(t *testing.T) {
	plg := mustCompilePlugin(t, minimalOneStepYAML())
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyFinalPage, "https://example.com/page")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/img.jpg", nil)
	err := plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/page", req.Header.Get("Referer"))
}

// --- resolveRequestURL with rawURL ---

func TestResolveRequestURL(t *testing.T) {
	pathTmpl, _ := compileTemplate("/search/${number}")
	spec := &compiledRequest{path: pathTmpl}
	u, err := resolveRequestURL(spec, &evalContext{number: "ABC", host: "https://example.com"})
	require.NoError(t, err)
	assert.Contains(t, u, "/search/ABC")

	urlTmpl, _ := compileTemplate("https://other.com/${number}")
	spec2 := &compiledRequest{rawURL: urlTmpl}
	u2, err := resolveRequestURL(spec2, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.Equal(t, "https://other.com/ABC", u2)
}

// --- OnDecodeHTTPData with unsupported charset ---

func TestOnDecodeHTTPData_UnsupportedCharset(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  response:
    decode_charset: "windows-1252"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	_, _, err := plg.OnDecodeHTTPData(ctx, []byte(`<html><title>T</title></html>`))
	require.Error(t, err)
}

// --- renderFormBody / renderJSONBody / renderRawBody ---

func TestRenderFormBody(t *testing.T) {
	tmpl, _ := compileTemplate("${number}")
	body := &compiledRequestBody{kind: "form", values: map[string]*template{"q": tmpl}}
	reader, err := renderFormBody(body, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.NotNil(t, reader)
}

func TestRenderJSONBody(t *testing.T) {
	tmpl, _ := compileTemplate("${number}")
	body := &compiledRequestBody{kind: "json", values: map[string]*template{"q": tmpl}}
	reader, err := renderJSONBody(body, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.NotNil(t, reader)
}

func TestRenderRawBody(t *testing.T) {
	tmpl, _ := compileTemplate("data-${number}")
	body := &compiledRequestBody{kind: "raw", content: tmpl}
	reader, err := renderRawBody(body, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.NotNil(t, reader)

	body2 := &compiledRequestBody{kind: "raw", content: nil}
	reader2, err := renderRawBody(body2, &evalContext{})
	require.NoError(t, err)
	assert.Nil(t, reader2)
}

// --- buildRequestBodyReader ---

func TestBuildRequestBodyReader_NilBody(t *testing.T) {
	spec := &compiledRequest{}
	reader, err := buildRequestBodyReader(spec, &evalContext{})
	require.NoError(t, err)
	assert.Nil(t, reader)
}

// --- condition evalTwoStringCondition all branches ---

func TestEvalTwoStringCondition_AllBranches(t *testing.T) {
	tests := []struct {
		name     string
		condName string
		left     string
		right    string
		expect   bool
	}{
		{name: "contains_true", condName: "contains", left: "hello world", right: "world", expect: true},
		{name: "contains_false", condName: "contains", left: "hello", right: "world", expect: false},
		{name: "equals_true", condName: "equals", left: "abc", right: "abc", expect: true},
		{name: "equals_false", condName: "equals", left: "abc", right: "def", expect: false},
		{name: "starts_with_true", condName: "starts_with", left: "abc", right: "ab", expect: true},
		{name: "starts_with_false", condName: "starts_with", left: "abc", right: "bc", expect: false},
		{name: "ends_with_true", condName: "ends_with", left: "abc", right: "bc", expect: true},
		{name: "ends_with_false", condName: "ends_with", left: "abc", right: "ab", expect: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			leftTmpl, _ := compileTemplate(tc.left)
			rightTmpl, _ := compileTemplate(tc.right)
			args := []compiledConditionArg{
				{kind: "string", template: leftTmpl},
				{kind: "string", template: rightTmpl},
			}
			ok, err := evalTwoStringCondition(tc.condName, args, &evalContext{})
			require.NoError(t, err)
			assert.Equal(t, tc.expect, ok)
		})
	}
}

// --- SyncBundle / BuildRegisterContext ---

func TestSyncBundle(_ *testing.T) {
	plugins := map[string][]byte{
		"test-plugin": []byte(minimalOneStepYAML()),
	}
	SyncBundle(plugins)
}

func TestBuildRegisterContext(t *testing.T) {
	plugins := map[string][]byte{
		"b-plugin": []byte(minimalOneStepYAML()),
		"a-plugin": []byte(minimalOneStepYAML()),
	}
	ctx := BuildRegisterContext(plugins)
	assert.NotNil(t, ctx)
}

// --- selectedHost ---

func TestSelectedHost_WithContext(t *testing.T) {
	ctx := &evalContext{host: "https://custom.com"}
	assert.Equal(t, "https://custom.com", selectedHost(ctx, []string{"https://default.com"}))
}

func TestSelectedHost_EmptyHosts(t *testing.T) {
	assert.Equal(t, "", selectedHost(nil, nil))
}

// --- OnDecodeHTTPData with postprocess ---

func TestOnDecodeHTTPData_WithPostprocess(t *testing.T) {
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
    title: "${meta.title} (post)"
  defaults:
    title_lang: ja
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`<html><head><title>OrigTitle</title></head></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, mv.Title, "post")
}

// --- handleResponse error paths ---

func TestHandleResponse_SelectorCountMismatch(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
      - name: text
        kind: xpath
        expr: "//span/text()"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		body := `<html><body><a href="/1">A</a><a href="/2">B</a><span>only one</span></body></html>`
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(body)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- handleResponse with no match ---

func TestHandleResponse_NoMatch(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
      - name: text
        kind: xpath
        expr: "//a/text()"
    match:
      mode: and
      conditions:
        - 'contains("${item.text}", "NOPE")'
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		body := `<html><body><a href="/1">A</a><a href="/2">B</a></body></html>`
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(body)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- followNextRequest error ---

func TestHandleResponse_NextRequestFails(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><a href="/detail/1">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 404,
			Body:       nopCloser([]byte("not found")),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- OnPrecheckResponse: all branches ---

func TestOnPrecheckResponse_AllBranches(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		status   int
		expected bool
	}{
		{
			name: "no_final_request_404",
			yaml: `
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
`,
			status:   404,
			expected: false,
		},
		{
			name: "not_found_status_code",
			yaml: `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  not_found_status_codes: [302, 301]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`,
			status:   302,
			expected: false,
		},
		{
			name: "accept_status_codes_match",
			yaml: `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  accept_status_codes: [200, 201]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`,
			status:   201,
			expected: true,
		},
		{
			name: "no_accept_codes_with_404",
			yaml: `
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
`,
			status:   404,
			expected: false,
		},
		{
			name: "no_accept_codes_non_404",
			yaml: `
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
`,
			status:   200,
			expected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plg := mustCompilePlugin(t, tc.yaml)
			ctx := pluginapi.InitContainer(context.Background())
			rsp := &http.Response{StatusCode: tc.status, Body: nopCloser(nil)}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
			ok, err := plg.OnPrecheckResponse(ctx, req, rsp)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, ok)
		})
	}
}

// --- OnPrecheckResponse: accept_status_codes reject with error ---

func TestOnPrecheckResponse_AcceptCodesReject(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  accept_status_codes: [200]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	rsp := &http.Response{StatusCode: 500, Body: nopCloser(nil)}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	ok, err := plg.OnPrecheckResponse(ctx, req, rsp)
	require.Error(t, err)
	assert.False(t, ok)
}

// --- buildRequest: empty host fallback ---

func TestBuildRequest_EmptyHostFallback(t *testing.T) {
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
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	evalCtx := &evalContext{number: "ABC-123"}
	req, err := plg.buildRequest(ctx, plg.spec.request, evalCtx)
	require.NoError(t, err)
	assert.Contains(t, req.URL.String(), "example.com")
}

// --- buildRequest: with body (form) ---

func TestBuildRequest_WithFormBody(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: POST
  path: /search
  body:
    kind: form
    values:
      q: "${number}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
}

// --- buildRequest: with query params ---

func TestBuildRequest_WithQueryParams(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search
  query:
    q: "${number}"
    page: "1"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.Contains(t, req.URL.RawQuery, "q=ABC-123")
}

// --- buildRequest: with headers and cookies ---

func TestBuildRequest_WithHeadersAndCookies(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  headers:
    X-Custom: "test-val"
  cookies:
    session: "abc"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.Equal(t, "test-val", req.Header.Get("X-Custom"))
}

// --- compilePlugin: all branches ---

func TestCompilePlugin_ErrorInScrape(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/"},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInRequest(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInMultiRequest(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts: []string{"https://example.com"},
		MultiRequest: &MultiRequestSpec{
			Request: &RequestSpec{Method: "GET", Path: "/"},
		},
		Scrape: &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInWorkflow(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "two-step",
		Hosts:    []string{"https://example.com"},
		Request:  &RequestSpec{Method: "GET", Path: "/"},
		Workflow: &WorkflowSpec{SearchSelect: &SearchSelectWorkflowSpec{}},
		Scrape:   &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInPostprocess(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:       []string{"https://example.com"},
		Request:     &RequestSpec{Method: "GET", Path: "/"},
		Scrape:      &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
		Postprocess: &PostprocessSpec{Assign: map[string]string{"title": "${invalid_unclosed"}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

func TestCompilePlugin_ErrorInPrecheck(t *testing.T) {
	spec := &PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:    []string{"https://example.com"},
		Precheck: &PrecheckSpec{Variables: map[string]string{"x": "${invalid_unclosed"}},
		Request:  &RequestSpec{Method: "GET", Path: "/"},
		Scrape:   &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	}
	_, err := compilePlugin(spec)
	require.Error(t, err)
}

// --- Condition Eval regex_match ---

func TestConditionEval_RegexMatch(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  unique: true
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'regex_match("${candidate}", "^[A-Z]+-\\d+$")'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><title>T</title></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	defer func() { _ = rsp.Body.Close() }()
	assert.Equal(t, 200, rsp.StatusCode)
}

func TestConditionEval_RegexMatchFail(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  unique: true
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'regex_match("${candidate}", "^NOPE-\\d+$")'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><title>T</title></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- OnMakeHTTPRequest with nil request but multiRequest ---

func TestOnMakeHTTPRequest_NilRequest_WithMultiRequest(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	req, err := plg.OnMakeHTTPRequest(ctx, "ABC-123")
	require.NoError(t, err)
	assert.NotNil(t, req)
}

// --- OnDecodeHTTPData_JSON_InvalidJSON ---

func TestOnDecodeHTTPData_JSON_InvalidJSON(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: $.title
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	_, _, err := plg.OnDecodeHTTPData(ctx, []byte(`not valid json`))
	require.Error(t, err)
}

// --- OnDecodeHTTPData unsupported format ---

func TestOnDecodeHTTPData_UnsupportedFormat2(t *testing.T) {
	spec, err := compilePlugin(&PluginSpec{
		Version: 1, Name: "test", Type: "one-step",
		Hosts:   []string{"https://example.com"},
		Request: &RequestSpec{Method: "GET", Path: "/search/${number}"},
		Scrape:  &ScrapeSpec{Format: "html", Fields: map[string]*FieldSpec{"title": {Selector: &SelectorSpec{Kind: "xpath", Expr: "//title/text()"}}}},
	})
	require.NoError(t, err)
	spec.scrape.format = "xml"
	plg := &SearchPlugin{spec: spec}
	ctx := pluginapi.InitContainer(context.Background())
	_, _, err = plg.OnDecodeHTTPData(ctx, []byte("<xml>data</xml>"))
	require.Error(t, err)
}

// --- checkAcceptedStatus: notFoundStatusCodes and no acceptStatusCodes non-200 ---

func TestCheckAcceptedStatus_NotFoundCode(t *testing.T) {
	spec := &compiledRequest{notFoundStatusCodes: []int{302}, acceptStatusCodes: nil}
	err := checkAcceptedStatus(spec, 302)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStatusCodeNotFound)
}

func TestCheckAcceptedStatus_NoAcceptCodesNon200(t *testing.T) {
	spec := &compiledRequest{acceptStatusCodes: nil}
	err := checkAcceptedStatus(spec, 500)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStatusCodeNotAccepted)
}

func TestCheckAcceptedStatus_AcceptCodesReject(t *testing.T) {
	spec := &compiledRequest{acceptStatusCodes: []int{200, 201}}
	err := checkAcceptedStatus(spec, 500)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStatusCodeNotAccepted)
}

// --- compilePlugin: precheck with variables ---

func TestCompilePlugin_WithPrecheckVariables(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
precheck:
  variables:
    upper_number: "${to_upper(${number})}"
request:
  method: GET
  path: /search/${vars.upper_number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "abc-123")
	ok, err := plg.OnPrecheckRequest(ctx, "abc-123")
	require.NoError(t, err)
	assert.True(t, ok)
	v, _ := pluginapi.GetContainerValue(ctx, ctxVarKey("upper_number"))
	assert.Equal(t, "ABC-123", v)
}

// --- OnDecodeHTTPData with assign postprocess ---

func TestOnDecodeHTTPData_WithAssignPostprocess(t *testing.T) {
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
    number:
      selector:
        kind: xpath
        expr: //span[@class="num"]/text()
postprocess:
  assign:
    title: "${meta.title} [edited]"
  defaults:
    title_lang: ja
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	data := []byte(`<html><head><title>Orig</title></head><body><span class="num">ABC-123</span></body></html>`)
	mv, ok, err := plg.OnDecodeHTTPData(ctx, data)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, mv.Title, "[edited]")
}

// --- OnDecorateMediaRequest with cookies ---

func TestOnDecorateMediaRequest_WithCookies(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  cookies:
    session: "token123"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/img.jpg", nil)
	err := plg.OnDecorateMediaRequest(ctx, req)
	require.NoError(t, err)
	cookies := req.Cookies()
	assert.NotEmpty(t, cookies)
}

// --- followNextRequest with status rejected ---

func TestFollowNextRequest_StatusRejected(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
      response:
        accept_status_codes: [200]
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><a href="/detail">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 403,
			Body:       nopCloser([]byte("forbidden")),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- handleRequest invoker error ---

func TestHandleRequest_InvokerError(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network error")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- multiRequest.handle: invoker error ---

func TestMultiRequestHandle_InvokerError(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection timeout")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- ConditionGroup OR mode evaluation ---

func TestConditionGroup_OrMode_FirstTrue(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: or
    conditions:
      - 'contains("${candidate}", "ABC")'
      - 'contains("${candidate}", "NOPE")'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	defer func() { _ = rsp.Body.Close() }()
}

// --- decodeHTML with required string field empty ---

func TestOnDecodeHTTPData_HTML_RequiredStringEmpty(t *testing.T) {
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
      required: true
      selector:
        kind: xpath
        expr: //h1[@class="missing"]/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	_, ok, err := plg.OnDecodeHTTPData(ctx, []byte(`<html><body><h1>Found</h1></body></html>`))
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- decodeJSON with required string field empty ---

func TestOnDecodeHTTPData_JSON_RequiredStringEmpty(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
scrape:
  format: json
  fields:
    title:
      required: true
      selector:
        kind: jsonpath
        expr: $.missing_field
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	_, ok, err := plg.OnDecodeHTTPData(ctx, []byte(`{"other": "value"}`))
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- multiRequest + workflow: handleResponse path ---

func TestOnHandleHTTPRequest_MultiRequestWithWorkflow2(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div[@class=''found'']"))'
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><div class="found">ok</div><a href="/detail/1">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><h1>DetailTitle</h1></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	defer func() { _ = rsp.Body.Close() }()
}

// --- evalTemplateExpr tests ---

func TestEvalTemplateExpr_ResolveVar(t *testing.T) {
	ctx := &evalContext{number: "ABC-123"}
	v, err := evalTemplateExpr("number", ctx)
	require.NoError(t, err)
	assert.Equal(t, "ABC-123", v)
}

func TestEvalTemplateExpr_FunctionCall(t *testing.T) {
	ctx := &evalContext{number: "abc-123"}
	v, err := evalTemplateExpr(`to_upper("hello")`, ctx)
	require.NoError(t, err)
	assert.Equal(t, "HELLO", v)
}

func TestEvalTemplateExpr_Empty(t *testing.T) {
	ctx := &evalContext{}
	v, err := evalTemplateExpr("", ctx)
	require.NoError(t, err)
	assert.Equal(t, "", v)
}

// --- readResponseBody with charset ---

func TestReadResponseBody_WithCharset(t *testing.T) {
	rsp := &http.Response{
		StatusCode: 200,
		Body:       nopCloser([]byte(`<html><body>hello</body></html>`)),
		Header:     make(http.Header),
	}
	body, node, err := readResponseBody(rsp, "utf-8")
	require.NoError(t, err)
	assert.Contains(t, body, "hello")
	assert.NotNil(t, node)
}

// --- handleResponse readResponseBody error (unsupported charset) ---

func TestHandleResponse_ReadBodyError(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
  response:
    decode_charset: "unknown-charset"
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><a href="/1">a</a></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- followNextRequest invoker error ---

func TestFollowNextRequest_InvokerError(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
request:
  method: GET
  path: /search/${number}
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 200,
				Body:       nopCloser([]byte(`<html><body><a href="/detail">link</a></body></html>`)),
				Header:     make(http.Header),
			}, nil
		}
		return nil, fmt.Errorf("detail page unreachable")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/search/ABC-123", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}

// --- multiRequest.handle with status rejected then skip ---

func TestMultiRequestHandle_StatusRejected(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["bad", "${number}"]
  request:
    method: GET
    path: /search/${candidate}
    response:
      accept_status_codes: [200]
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div[@class=''found'']"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 404,
				Body:       nopCloser([]byte("not found")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><div class="found">ok</div><title>Found</title></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	defer func() { _ = rsp.Body.Close() }()
}

// --- multiRequest.handle: unique dedup ---

func TestMultiRequestHandle_UniqueDedup(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: one-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}", "${number}"]
  unique: true
  request:
    method: GET
    path: /search/${candidate}
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	callCount := 0
	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: 200,
			Body:       nopCloser([]byte(`<html><body><div>ok</div></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	require.NoError(t, err)
	defer func() { _ = rsp.Body.Close() }()
	assert.Equal(t, 1, callCount)
}

// --- handleResponse with checkBaseResponseStatus rejection (multi_request path) ---

func TestHandleResponse_BaseStatusRejected(t *testing.T) {
	yamlStr := `
version: 1
name: test
type: two-step
hosts: ["https://example.com"]
multi_request:
  candidates: ["${number}"]
  request:
    method: GET
    path: /search/${candidate}
    response:
      accept_status_codes: [200]
  success_when:
    mode: and
    conditions:
      - 'selector_exists(xpath("//div"))'
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: "//a/@href"
    return: "${item.link}"
    next_request:
      method: GET
      path: "${value}"
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
`
	plg := mustCompilePlugin(t, yamlStr)
	ctx := pluginapi.InitContainer(context.Background())
	ctx = meta.SetNumberID(ctx, "ABC-123")
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://example.com")

	invoker := func(_ context.Context, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Body:       nopCloser([]byte(`<html><body><div>ok</div><a href="/x">x</a></body></html>`)),
			Header:     make(http.Header),
		}, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if rsp != nil && rsp.Body != nil {
		defer func() { _ = rsp.Body.Close() }()
	}
	require.Error(t, err)
}
