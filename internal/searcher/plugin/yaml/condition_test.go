package yaml

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/antchfx/htmlquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func TestCompileConditionGroup_InvalidCondition(t *testing.T) {
	spec := &ConditionGroupSpec{
		Mode:       "and",
		Conditions: []string{"not_a_call"},
	}
	_, err := compileConditionGroup(spec)
	require.Error(t, err)
}

func TestCompileTwoStringArgs_WrongCount(t *testing.T) {
	_, err := compileTwoStringArgs("contains", []string{"one"})
	require.ErrorIs(t, err, errConditionExpects2Args)
}

func TestCompileTwoStringArgs_NonString(t *testing.T) {
	_, err := compileTwoStringArgs("contains", []string{"unquoted", `"b"`})
	require.ErrorIs(t, err, errConditionExpectsStringArgs)
}

func TestCompileRegexMatchArgs_NonStringFirst(t *testing.T) {
	_, err := compileRegexMatchArgs([]string{"unquoted", `"pattern"`})
	require.ErrorIs(t, err, errRegexMatchExpectsStringFirst)
}

func TestCompileRegexMatchArgs_UnquotedPattern(t *testing.T) {
	_, err := compileRegexMatchArgs([]string{`"text"`, "unquoted"})
	require.ErrorIs(t, err, errRegexMatchExpectsQuoted)
}

func TestCompileRegexMatchArgs_InvalidRegex(t *testing.T) {
	_, err := compileRegexMatchArgs([]string{`"text"`, `"[invalid"`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compile regex failed")
}

func TestCompileQuotedStringArg_Unquoted(t *testing.T) {
	tmpl, ok, err := compileQuotedStringArg("unquoted")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, tmpl)
}

func TestCompileQuotedStringArg_InvalidTemplate(t *testing.T) {
	_, _, err := compileQuotedStringArg(`"${unterminated"`)
	require.Error(t, err)
}

func TestCompileSelectorLiteral_NotXpath(t *testing.T) {
	_, err := compileSelectorLiteral(`css("selector")`)
	require.ErrorIs(t, err, errSelectorExistsExpectsXpath)
}

func TestCompileSelectorLiteral_NotACall(t *testing.T) {
	_, err := compileSelectorLiteral(`"just a string"`)
	require.ErrorIs(t, err, errSelectorExistsExpectsXpath)
}

func TestCompileSelectorLiteral_UnquotedArg(t *testing.T) {
	_, err := compileSelectorLiteral(`xpath(unquoted)`)
	require.ErrorIs(t, err, errXpathExpectsQuotedString)
}

func TestCompileSelectorLiteral_TooManyArgs(t *testing.T) {
	_, err := compileSelectorLiteral(`xpath("a", "b")`)
	require.ErrorIs(t, err, errSelectorExistsExpectsXpath)
}

func TestConditionGroupEval_OrMode(t *testing.T) {
	tests := []struct {
		name       string
		conditions []string
		expect     bool
	}{
		{
			name:       "first_true",
			conditions: []string{`contains("abc", "a")`, `contains("abc", "z")`},
			expect:     true,
		},
		{
			name:       "second_true",
			conditions: []string{`contains("abc", "z")`, `contains("abc", "b")`},
			expect:     true,
		},
		{
			name:       "all_false",
			conditions: []string{`contains("abc", "z")`, `contains("abc", "q")`},
			expect:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := compileConditionGroup(&ConditionGroupSpec{
				Mode:       "or",
				Conditions: tt.conditions,
			})
			require.NoError(t, err)
			ok, err := g.Eval(&evalContext{}, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, ok)
		})
	}
}

func TestConditionGroupEval_And_FirstFails(t *testing.T) {
	g, err := compileConditionGroup(&ConditionGroupSpec{
		Mode:       "and",
		Conditions: []string{`contains("abc", "z")`, `contains("abc", "a")`},
	})
	require.NoError(t, err)
	ok, err := g.Eval(&evalContext{}, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestConditionGroupEval_NilGroup(t *testing.T) {
	var g *compiledConditionGroup
	ok, err := g.Eval(&evalContext{}, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestEvalTwoStringCondition_AllNameBranches(t *testing.T) {
	tests := []struct {
		name   string
		fn     string
		left   string
		right  string
		expect bool
	}{
		{"contains_true", "contains", "hello world", "world", true},
		{"contains_false", "contains", "hello", "xyz", false},
		{"equals_true", "equals", "abc", "abc", true},
		{"equals_false", "equals", "abc", "xyz", false},
		{"starts_with_true", "starts_with", "hello", "hel", true},
		{"starts_with_false", "starts_with", "hello", "xyz", false},
		{"ends_with_true", "ends_with", "hello", "llo", true},
		{"ends_with_false", "ends_with", "hello", "xyz", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leftTmpl, err := compileTemplate(tt.left)
			require.NoError(t, err)
			rightTmpl, err := compileTemplate(tt.right)
			require.NoError(t, err)
			args := []compiledConditionArg{
				{kind: "string", template: leftTmpl},
				{kind: "string", template: rightTmpl},
			}
			ok, err := evalTwoStringCondition(tt.fn, args, &evalContext{})
			require.NoError(t, err)
			assert.Equal(t, tt.expect, ok)
		})
	}
}

func TestEvalTwoStringCondition_UnknownName(t *testing.T) {
	leftTmpl, err := compileTemplate("a")
	require.NoError(t, err)
	rightTmpl, err := compileTemplate("b")
	require.NoError(t, err)
	args := []compiledConditionArg{
		{kind: "string", template: leftTmpl},
		{kind: "string", template: rightTmpl},
	}
	_, err = evalTwoStringCondition("unknown_fn", args, &evalContext{})
	require.ErrorIs(t, err, errUnknownCondition)
}

func TestEvalSelectorExists_WithNode(t *testing.T) {
	htmlStr := `<html><body><div class="test">ok</div></body></html>`
	node, err := htmlquery.Parse(strings.NewReader(htmlStr))
	require.NoError(t, err)

	sel := &compiledSelector{kind: "xpath", expr: `//div[@class="test"]`}
	args := []compiledConditionArg{{kind: "selector", selector: sel}}
	ok, err := evalSelectorExists(args, &evalContext{}, node)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestEvalSelectorExists_NoMatch(t *testing.T) {
	htmlStr := `<html><body><span>nope</span></body></html>`
	node, err := htmlquery.Parse(strings.NewReader(htmlStr))
	require.NoError(t, err)

	sel := &compiledSelector{kind: "xpath", expr: `//div[@class="missing"]`}
	args := []compiledConditionArg{{kind: "selector", selector: sel}}
	ok, err := evalSelectorExists(args, &evalContext{}, node)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEvalSelectorExists_EmptyBody_NilNode(t *testing.T) {
	sel := &compiledSelector{kind: "xpath", expr: `//div`}
	args := []compiledConditionArg{{kind: "selector", selector: sel}}
	ok, err := evalSelectorExists(args, &evalContext{body: "  "}, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEvalSelectorExists_ParseBodyFromCtx(t *testing.T) {
	sel := &compiledSelector{kind: "xpath", expr: `//div[@class="found"]`}
	args := []compiledConditionArg{{kind: "selector", selector: sel}}
	ctx := &evalContext{body: `<html><body><div class="found">yes</div></body></html>`}
	ok, err := evalSelectorExists(args, ctx, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCompiledCondition_Eval_UnknownName(t *testing.T) {
	cond := &compiledCondition{name: "bogus_function"}
	_, err := cond.Eval(&evalContext{}, nil)
	require.ErrorIs(t, err, errUnknownCondition)
}

func TestCompiledCondition_Eval_RegexMatch(t *testing.T) {
	cond, err := compileCondition(`regex_match("hello-world-123", "^hello.*\\d+$")`)
	require.NoError(t, err)
	ok, err := cond.Eval(&evalContext{}, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCompiledCondition_Eval_SelectorExists_WithHTMLNode(t *testing.T) {
	cond, err := compileCondition(`selector_exists(xpath("//p"))`)
	require.NoError(t, err)

	htmlStr := `<html><body><p>hello</p></body></html>`
	node, err := htmlquery.Parse(strings.NewReader(htmlStr))
	require.NoError(t, err)

	ok, err := cond.Eval(&evalContext{}, node)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestConditionGroupEval_OrMode_ErrorPropagation(t *testing.T) {
	cond := &compiledCondition{name: "unknown_cond"}
	g := &compiledConditionGroup{
		mode:       conditionModeOr,
		conditions: []*compiledCondition{cond},
	}
	_, err := g.Eval(&evalContext{}, nil)
	require.Error(t, err)
}

func TestConditionGroupEval_AndMode_ErrorPropagation(t *testing.T) {
	cond := &compiledCondition{name: "unknown_cond"}
	g := &compiledConditionGroup{
		mode:       conditionModeAnd,
		conditions: []*compiledCondition{cond},
	}
	_, err := g.Eval(&evalContext{}, nil)
	require.Error(t, err)
}

func TestCompileTwoStringArgs_FirstArgNotQuoted(t *testing.T) {
	_, err := compileTwoStringArgs("contains", []string{"unquoted", `"quoted"`})
	require.ErrorIs(t, err, errConditionExpectsStringArgs)
}

func TestCompileTwoStringArgs_SecondArgNotQuoted(t *testing.T) {
	_, err := compileTwoStringArgs("contains", []string{`"quoted"`, "unquoted"})
	require.ErrorIs(t, err, errConditionExpectsStringArgs)
}

func TestCompileRegexMatchArgs_FirstArgCompileError(t *testing.T) {
	_, err := compileRegexMatchArgs([]string{`"${bad_fn()}"`, `"pat"`})
	require.Error(t, err)
}

func TestCompileSelectorLiteral_NameNotXpath(t *testing.T) {
	_, err := compileSelectorLiteral(`css(".link")`)
	require.ErrorIs(t, err, errSelectorExistsExpectsXpath)
}

func TestCompileSelectorLiteral_TwoArgs(t *testing.T) {
	_, err := compileSelectorLiteral(`xpath("//a", "extra")`)
	require.ErrorIs(t, err, errSelectorExistsExpectsXpath)
}

func TestCompileCondition_SelectorExistsWrongArgCount(t *testing.T) {
	_, err := compileCondition(`selector_exists(xpath("//a"), xpath("//b"))`)
	require.ErrorIs(t, err, errSelectorExistsExpects1Arg)
}

func TestCompileCondition_SelectorExistsInvalidLiteral(t *testing.T) {
	_, err := compileCondition(`selector_exists(not_xpath("//a"))`)
	require.Error(t, err)
}

func TestEvalTwoStringCondition_LeftRenderError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	goodTmpl, err := compileTemplate("hello")
	require.NoError(t, err)
	args := []compiledConditionArg{
		{kind: "string", template: badTmpl},
		{kind: "string", template: goodTmpl},
	}
	_, err = evalTwoStringCondition("contains", args, &evalContext{})
	require.Error(t, err)
}

func TestEvalTwoStringCondition_RightRenderError(t *testing.T) {
	goodTmpl, err := compileTemplate("hello")
	require.NoError(t, err)
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	args := []compiledConditionArg{
		{kind: "string", template: goodTmpl},
		{kind: "string", template: badTmpl},
	}
	_, err = evalTwoStringCondition("contains", args, &evalContext{})
	require.Error(t, err)
}

func TestEvalSelectorExists_EmptyBodyAndNilNode(t *testing.T) {
	sel := &compiledSelector{kind: "xpath", expr: "//div"}
	args := []compiledConditionArg{{kind: "selector", selector: sel}}
	ok, err := evalSelectorExists(args, &evalContext{body: ""}, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEvalSelectorExists_BodyParsed(t *testing.T) {
	sel := &compiledSelector{kind: "xpath", expr: "//div[@id='target']"}
	args := []compiledConditionArg{{kind: "selector", selector: sel}}
	ok, err := evalSelectorExists(args, &evalContext{body: `<html><body><div id="target">X</div></body></html>`}, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCompiledCondition_Eval_RegexMatchLeftError(t *testing.T) {
	badTmpl, err := compileTemplate("${vars.missing}")
	require.NoError(t, err)
	cond := &compiledCondition{
		name: "regex_match",
		args: []compiledConditionArg{
			{kind: "string", template: badTmpl},
			{kind: "regex", regex: nil},
		},
	}
	_, err = cond.Eval(&evalContext{}, nil)
	require.Error(t, err)
}

func TestConditionGroupEval_Or_AllFalse(t *testing.T) {
	cond1, err := compileCondition(`contains("abc", "xyz")`)
	require.NoError(t, err)
	cond2, err := compileCondition(`equals("abc", "def")`)
	require.NoError(t, err)
	g := &compiledConditionGroup{
		mode:       conditionModeOr,
		conditions: []*compiledCondition{cond1, cond2},
	}
	ok, err := g.Eval(&evalContext{}, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestConditionGroupEval_And_AllTrue(t *testing.T) {
	cond1, err := compileCondition(`contains("abc", "ab")`)
	require.NoError(t, err)
	cond2, err := compileCondition(`starts_with("abc", "a")`)
	require.NoError(t, err)
	g := &compiledConditionGroup{
		mode:       conditionModeAnd,
		conditions: []*compiledCondition{cond1, cond2},
	}
	ok, err := g.Eval(&evalContext{}, nil)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestConditionGroupEval_Or_ErrorInSecond(t *testing.T) {
	cond1, err := compileCondition(`contains("abc", "xyz")`)
	require.NoError(t, err)
	cond2 := &compiledCondition{name: "unknown_cond"}
	g := &compiledConditionGroup{
		mode:       conditionModeOr,
		conditions: []*compiledCondition{cond1, cond2},
	}
	_, err = g.Eval(&evalContext{}, nil)
	require.Error(t, err)
}

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
