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
	_, _, _, err := parseCall(`fn("unterminated)`) //nolint:dogsled // 测试只关心前若干返回值
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

	_, _, ok, _ = parseCall("not_a_call") //nolint:dogsled // 测试只关心前若干返回值
	assert.False(t, ok)

	_, _, ok, _ = parseCall("123(a)") //nolint:dogsled // 测试只关心前若干返回值
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

func TestSplitArgs_Errors(t *testing.T) {
	_, err := splitArgs(`"unterminated`)
	assert.Error(t, err)

	_, err = splitArgs(`(unbalanced`)
	assert.Error(t, err)

	_, err = splitArgs(`)extra_close`)
	assert.Error(t, err)
}

// --- movieMetaStringMap ---

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
