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
