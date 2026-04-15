package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTemplateExpr_Empty(t *testing.T) {
	err := validateTemplateExpr("")
	require.NoError(t, err)
}

func TestValidateTemplateExpr_UnknownFunction(t *testing.T) {
	err := validateTemplateExpr(`bogus_fn("a")`)
	require.ErrorIs(t, err, errUnknownTemplateFunction)
}

func TestValidateTemplateExpr_InvalidExpr(t *testing.T) {
	err := validateTemplateExpr(`not_a_var_or_func`)
	require.ErrorIs(t, err, errInvalidTemplateExpr)
}

func TestValidateTemplateExpr_InvalidArgInFunction(t *testing.T) {
	err := validateTemplateExpr(`to_upper("${unterminated")`)
	require.Error(t, err)
}

func TestValidateTemplateArg_Empty(t *testing.T) {
	err := validateTemplateArg("")
	require.NoError(t, err)
}

func TestValidateTemplateArg_QuotedWithTemplate(t *testing.T) {
	err := validateTemplateArg(`"${number}"`)
	require.NoError(t, err)
}

func TestValidateTemplateArg_NestedTemplateExpr(t *testing.T) {
	err := validateTemplateArg(`${number}`)
	require.NoError(t, err)
}

func TestValidateTemplateArg_ContainsTemplate(t *testing.T) {
	err := validateTemplateArg(`prefix-${number}-suffix`)
	require.NoError(t, err)
}

func TestValidateTemplateArg_NestedCall(t *testing.T) {
	err := validateTemplateArg(`to_upper("abc")`)
	require.NoError(t, err)
}

func TestValidateTemplateArg_VariableRef(t *testing.T) {
	err := validateTemplateArg("number")
	require.NoError(t, err)
}

func TestValidateTemplateArg_PlainLiteral(t *testing.T) {
	err := validateTemplateArg("just_text_123")
	require.NoError(t, err)
}

func TestRenderTemplate_NoTemplate(t *testing.T) {
	result, err := renderTemplate("plain text", &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "plain text", result)
}

func TestRenderTemplate_ErrorInExpr(t *testing.T) {
	_, err := renderTemplate("${unknown_var}", &evalContext{})
	require.Error(t, err)
}

func TestEvalTemplateExpr_EmptyExpr(t *testing.T) {
	result, err := evalTemplateExpr("", &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEvalTemplateExpr_FuncCall(t *testing.T) {
	result, err := evalTemplateExpr(`to_upper("abc")`, &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "ABC", result)
}

func TestEvalTemplateExpr_VarRef(t *testing.T) {
	result, err := evalTemplateExpr("number", &evalContext{number: "X"})
	require.NoError(t, err)
	assert.Equal(t, "X", result)
}

func TestEvalTemplateExpr_ParseCallError(t *testing.T) {
	_, err := evalTemplateExpr(`fn("unterminated`, &evalContext{})
	require.Error(t, err)
}

func TestEvalTemplateArg_EmptyString(t *testing.T) {
	result, err := evalTemplateArg("", &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEvalTemplateArg_QuotedString(t *testing.T) {
	result, err := evalTemplateArg(`"hello"`, &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestEvalTemplateArg_TemplateExpr(t *testing.T) {
	result, err := evalTemplateArg(`${number}`, &evalContext{number: "ABC"})
	require.NoError(t, err)
	assert.Equal(t, "ABC", result)
}

func TestEvalTemplateArg_ContainsTemplate(t *testing.T) {
	result, err := evalTemplateArg(`prefix-${number}`, &evalContext{number: "X"})
	require.NoError(t, err)
	assert.Equal(t, "prefix-X", result)
}

func TestEvalTemplateArg_NestedCall(t *testing.T) {
	result, err := evalTemplateArg(`to_lower("ABC")`, &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "abc", result)
}

func TestEvalTemplateArg_VarRef(t *testing.T) {
	result, err := evalTemplateArg("number", &evalContext{number: "Z"})
	require.NoError(t, err)
	assert.Equal(t, "Z", result)
}

func TestEvalTemplateArg_PlainLiteral(t *testing.T) {
	result, err := evalTemplateArg("random_text", &evalContext{})
	require.NoError(t, err)
	assert.Equal(t, "random_text", result)
}

func TestEvalTemplateArg_NestedCallWithError(t *testing.T) {
	_, err := evalTemplateArg(`fn("unterminated)`, &evalContext{})
	require.Error(t, err)
}

func TestEvalBuildURL_WrongArgCount(t *testing.T) {
	_, err := evalBuildURL([]string{"only_one"})
	require.ErrorIs(t, err, errBuildURLExpects2Args)
}

func TestEvalBuildURL_InvalidBaseURL(t *testing.T) {
	_, err := evalBuildURL([]string{"://bad", "/path"})
	require.Error(t, err)
}

func TestEvalBuildURL_InvalidRefURL(t *testing.T) {
	_, err := evalBuildURL([]string{"https://example.com", "://bad"})
	require.Error(t, err)
}

func TestEvalLastSegment_WrongArgCount(t *testing.T) {
	_, err := evalLastSegment([]string{"only_one"})
	require.ErrorIs(t, err, errLastSegmentExpects2Args)
}

func TestEvalLastSegment_EmptySep(t *testing.T) {
	result, err := evalLastSegment([]string{"abc", ""})
	require.NoError(t, err)
	assert.Equal(t, "abc", result)
}

func TestEvalLastSegment_Normal(t *testing.T) {
	result, err := evalLastSegment([]string{"a/b/c", "/"})
	require.NoError(t, err)
	assert.Equal(t, "c", result)
}

func TestResolveTemplateVar_AllVars(t *testing.T) {
	ctx := &evalContext{
		number:        "NUM",
		host:          "HOST",
		body:          "BODY",
		value:         "VAL",
		candidate:     "CAND",
		vars:          map[string]string{"x": "vx"},
		item:          map[string]string{"y": "vy"},
		itemVariables: map[string]string{"z": "vz"},
		meta:          map[string]string{"title": "T"},
	}
	tests := []struct {
		ref    string
		expect string
	}{
		{"number", "NUM"},
		{"host", "HOST"},
		{"body", "BODY"},
		{"value", "VAL"},
		{"candidate", "CAND"},
		{"vars.x", "vx"},
		{"item.y", "vy"},
		{"item_variables.z", "vz"},
		{"meta.title", "T"},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			result, err := resolveTemplateVar(tt.ref, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestResolveTemplateVar_Unknown(t *testing.T) {
	_, err := resolveTemplateVar("unknown_ref", &evalContext{})
	require.ErrorIs(t, err, errUnknownTemplateVariable)
}

func TestParseCall_NoOpenParen(t *testing.T) {
	_, _, ok, err := parseCall("noparen")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestParseCall_NonIdentifierName(t *testing.T) {
	_, _, ok, err := parseCall("123func(a)")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestParseCall_InvalidArgs(t *testing.T) {
	_, _, _, err := parseCall(`fn("unterminated)`) //nolint:dogsled
	require.Error(t, err)
}

func TestEvalTemplateArg_NestedCallError(t *testing.T) {
	_, err := evalTemplateArg(`bogus_fn("a")`, &evalContext{})
	require.Error(t, err)
}

func TestFindTemplateEnd_Nested(t *testing.T) {
	raw := "${to_upper(${number})}"
	end, err := findTemplateEnd(raw, 0)
	require.NoError(t, err)
	assert.Equal(t, len(raw)-1, end)
}

func TestFindTemplateEnd_Unterminated(t *testing.T) {
	_, err := findTemplateEnd("${number", 0)
	require.ErrorIs(t, err, errUnterminatedTemplate)
}
