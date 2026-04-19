package yaml

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalJSONPathStrings_SingleString(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"title":"hello"}`), &doc))
	values, err := evalJSONPathStrings(doc, "$.title")
	require.NoError(t, err)
	assert.Equal(t, []string{"hello"}, values)
}

func TestEvalJSONPathStrings_ArrayValues(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"items":["a","b","c"]}`), &doc))
	values, err := evalJSONPathStrings(doc, "$.items[*]")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, values)
}

func TestEvalJSONPathStrings_MissingKeyReturnsNil(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"other":"val"}`), &doc))
	values, err := evalJSONPathStrings(doc, "$.missing")
	require.NoError(t, err)
	assert.Nil(t, values)
}

func TestEvalJSONPathStrings_IndexOutOfBoundsError(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"arr":[1]}`), &doc))
	_, err := evalJSONPathStrings(doc, "$.arr[5]")
	require.Error(t, err)
}

func TestEvalJSONPathStrings_InvalidExprError(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"x":1}`), &doc))
	_, err := evalJSONPathStrings(doc, "$[invalid")
	require.Error(t, err)
}

func TestEvalJSONPathStrings_NumericValue(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"count":42}`), &doc))
	values, err := evalJSONPathStrings(doc, "$.count")
	require.NoError(t, err)
	assert.Equal(t, []string{"42"}, values)
}

func TestEvalJSONPathStrings_BoolValue(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"active":true}`), &doc))
	values, err := evalJSONPathStrings(doc, "$.active")
	require.NoError(t, err)
	assert.Equal(t, []string{"true"}, values)
}

func TestEvalJSONPathStrings_NullValue(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"val":null}`), &doc))
	values, err := evalJSONPathStrings(doc, "$.val")
	require.NoError(t, err)
	assert.Empty(t, values)
}

func TestFlattenJSONPathValue_NilValue(t *testing.T) {
	var out []string
	flattenJSONPathValue(nil, &out)
	assert.Nil(t, out)
}

func TestFlattenJSONPathValue_StringValue(t *testing.T) {
	var out []string
	flattenJSONPathValue("hello", &out)
	assert.Equal(t, []string{"hello"}, out)
}

func TestFlattenJSONPathValue_Float64Value(t *testing.T) {
	var out []string
	flattenJSONPathValue(float64(42), &out)
	assert.Equal(t, []string{"42"}, out)
}

func TestFlattenJSONPathValue_BoolValue(t *testing.T) {
	var out []string
	flattenJSONPathValue(true, &out)
	assert.Equal(t, []string{"true"}, out)
}

func TestFlattenJSONPathValue_NestedArray(t *testing.T) {
	var out []string
	flattenJSONPathValue([]any{"a", float64(1), true}, &out)
	assert.Equal(t, []string{"a", "1", "true"}, out)
}

func TestFlattenJSONPathValue_MapValue(t *testing.T) {
	var out []string
	m := map[string]any{"key": "val"}
	flattenJSONPathValue(m, &out)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "key")
}

func TestFlattenJSONPathValue_DecimalFloat(t *testing.T) {
	var out []string
	flattenJSONPathValue(3.14, &out)
	assert.Equal(t, []string{"3.14"}, out)
}

func TestIsJSONPathMissingError_NilInput(t *testing.T) {
	assert.False(t, isJSONPathMissingError(nil))
}

func TestEvalJSONPathStrings_ObjectField(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{"nested":{"key":"val"}}`), &doc))
	values, err := evalJSONPathStrings(doc, "$.nested")
	require.NoError(t, err)
	require.Len(t, values, 1)
	assert.Contains(t, values[0], "key")
}

func TestEvalJSONPathStrings(t *testing.T) {
	doc := map[string]any{
		"str":  "hello",
		"num":  42.0,
		"bool": true,
		"arr":  []any{"a", "b"},
		"nested": map[string]any{
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
