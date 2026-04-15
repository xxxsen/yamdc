package yaml

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParserSpec_UnmarshalYAML_Scalar(t *testing.T) {
	input := `"string"`
	var p ParserSpec
	err := yaml.Unmarshal([]byte(input), &p)
	require.NoError(t, err)
	assert.Equal(t, "string", p.Kind)
	assert.Empty(t, p.Layout)
}

func TestParserSpec_UnmarshalYAML_Mapping(t *testing.T) {
	input := "kind: time_format\nlayout: \"2006-01-02\""
	var p ParserSpec
	err := yaml.Unmarshal([]byte(input), &p)
	require.NoError(t, err)
	assert.Equal(t, "time_format", p.Kind)
	assert.Equal(t, "2006-01-02", p.Layout)
}

func TestParserSpec_UnmarshalYAML_InvalidNodeKind(t *testing.T) {
	input := "- item1\n- item2"
	var p ParserSpec
	err := yaml.Unmarshal([]byte(input), &p)
	require.Error(t, err)
	assert.ErrorIs(t, err, errInvalidParserNodeKind)
}

func TestParserSpec_UnmarshalYAML_InvalidScalar(t *testing.T) {
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "valid", Tag: "!!int"}
	var p ParserSpec
	err := p.UnmarshalYAML(node)
	require.Error(t, err)
}

func TestParserSpec_UnmarshalJSON_String(t *testing.T) {
	input := `"duration_default"`
	var p ParserSpec
	err := json.Unmarshal([]byte(input), &p)
	require.NoError(t, err)
	assert.Equal(t, "duration_default", p.Kind)
	assert.Empty(t, p.Layout)
}

func TestParserSpec_UnmarshalJSON_Object(t *testing.T) {
	input := `{"kind":"time_format","layout":"2006-01-02"}`
	var p ParserSpec
	err := json.Unmarshal([]byte(input), &p)
	require.NoError(t, err)
	assert.Equal(t, "time_format", p.Kind)
	assert.Equal(t, "2006-01-02", p.Layout)
}

func TestParserSpec_UnmarshalJSON_Empty(t *testing.T) {
	var p ParserSpec
	err := p.UnmarshalJSON(nil)
	require.NoError(t, err)

	err = p.UnmarshalJSON([]byte{})
	require.NoError(t, err)
}

func TestParserSpec_UnmarshalJSON_InvalidString(t *testing.T) {
	var p ParserSpec
	err := json.Unmarshal([]byte(`"unterminated`), &p)
	require.Error(t, err)
}

func TestParserSpec_UnmarshalJSON_InvalidObject(t *testing.T) {
	var p ParserSpec
	err := json.Unmarshal([]byte(`{"kind": 123}`), &p)
	require.Error(t, err)
}

func TestParserSpec_UnmarshalJSON_Number(t *testing.T) {
	var p ParserSpec
	err := p.UnmarshalJSON([]byte(`123`))
	require.Error(t, err)
}
