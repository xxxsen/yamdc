package yaml

import (
	"strings"
	"testing"

	"github.com/antchfx/htmlquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
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

func helperParseHTML(t *testing.T, s string) *html.Node {
	t.Helper()
	n, err := htmlquery.Parse(strings.NewReader(s))
	require.NoError(t, err)
	return n
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
