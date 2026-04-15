package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

type conditionMode string

const (
	conditionModeAnd conditionMode = "and"
	conditionModeOr  conditionMode = "or"
)

var (
	errInvalidConditionMode         = errors.New("invalid condition mode")
	errConditionsEmpty              = errors.New("conditions is empty")
	errInvalidCondition             = errors.New("invalid condition")
	errConditionExpects2Args        = errors.New("condition expects 2 arguments")
	errConditionExpectsStringArgs   = errors.New("condition expects string arguments")
	errRegexMatchExpects2Args       = errors.New("regex_match expects 2 arguments")
	errRegexMatchExpectsStringFirst = errors.New("regex_match expects string first argument")
	errRegexMatchExpectsQuoted      = errors.New("regex_match expects quoted regex pattern")
	errSelectorExistsExpects1Arg    = errors.New("selector_exists expects 1 argument")
	errUnknownConditionFunction     = errors.New("unknown condition function")
	errSelectorExistsExpectsXpath   = errors.New("selector_exists expects xpath(\"...\")")
	errXpathExpectsQuotedString     = errors.New("xpath expects quoted string")
	errUnknownCondition             = errors.New("unknown condition")
)

type compiledConditionGroup struct {
	mode        conditionMode
	conditions  []*compiledCondition
	expectCount int
}

type compiledCondition struct {
	name string
	args []compiledConditionArg
}

type compiledConditionArg struct {
	kind     string
	template *template
	regex    *regexp.Regexp
	selector *compiledSelector
}

func compileConditionGroup(spec *ConditionGroupSpec) (*compiledConditionGroup, error) {
	if spec == nil {
		return nil, nil //nolint:nilnil // nil spec means no condition group configured
	}
	mode := conditionMode(spec.Mode)
	if mode != conditionModeAnd && mode != conditionModeOr {
		return nil, fmt.Errorf("invalid condition mode: %s: %w", spec.Mode, errInvalidConditionMode)
	}
	if len(spec.Conditions) == 0 {
		return nil, errConditionsEmpty
	}
	out := &compiledConditionGroup{
		mode:        mode,
		conditions:  make([]*compiledCondition, 0, len(spec.Conditions)),
		expectCount: spec.ExpectCount,
	}
	for _, raw := range spec.Conditions {
		c, err := compileCondition(raw)
		if err != nil {
			return nil, err
		}
		out.conditions = append(out.conditions, c)
	}
	return out, nil
}

func compileTwoStringArgs(name string, args []string) ([]compiledConditionArg, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("%s: %w", name, errConditionExpects2Args)
	}
	out := make([]compiledConditionArg, 0, 2)
	for _, arg := range args {
		t, ok, err := compileQuotedStringArg(arg)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%s: %w", name, errConditionExpectsStringArgs)
		}
		out = append(out, compiledConditionArg{kind: "string", template: t})
	}
	return out, nil
}

func compileRegexMatchArgs(args []string) ([]compiledConditionArg, error) {
	if len(args) != 2 {
		return nil, errRegexMatchExpects2Args
	}
	left, ok, err := compileQuotedStringArg(args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errRegexMatchExpectsStringFirst
	}
	pattern, ok := unquoteArg(strings.TrimSpace(args[1]))
	if !ok {
		return nil, errRegexMatchExpectsQuoted
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile regex failed, err:%w", err)
	}
	return []compiledConditionArg{
		{kind: "string", template: left},
		{kind: "regex", regex: re},
	}, nil
}

func compileCondition(raw string) (*compiledCondition, error) {
	name, args, ok, err := parseCall(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("invalid condition: %s: %w", raw, errInvalidCondition)
	}
	out := &compiledCondition{name: name}
	switch name {
	case "contains", "equals", "starts_with", "ends_with":
		out.args, err = compileTwoStringArgs(name, args)
	case "regex_match":
		out.args, err = compileRegexMatchArgs(args)
	case "selector_exists":
		if len(args) != 1 {
			return nil, errSelectorExistsExpects1Arg
		}
		sel, selErr := compileSelectorLiteral(args[0])
		if selErr != nil {
			return nil, selErr
		}
		out.args = append(out.args, compiledConditionArg{kind: "selector", selector: sel})
	default:
		return nil, fmt.Errorf("unknown condition function: %s: %w", name, errUnknownConditionFunction)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func compileQuotedStringArg(raw string) (*template, bool, error) {
	value, ok := unquoteArg(strings.TrimSpace(raw))
	if !ok {
		return nil, false, nil
	}
	t, err := compileTemplate(value)
	if err != nil {
		return nil, false, err
	}
	return t, true, nil
}

func compileSelectorLiteral(raw string) (*compiledSelector, error) {
	name, args, ok, err := parseCall(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if !ok || name != "xpath" || len(args) != 1 {
		return nil, errSelectorExistsExpectsXpath
	}
	expr, ok := unquoteArg(strings.TrimSpace(args[0]))
	if !ok {
		return nil, errXpathExpectsQuotedString
	}
	return &compiledSelector{kind: "xpath", expr: expr}, nil
}

func (g *compiledConditionGroup) Eval(ctx *evalContext, node *html.Node) (bool, error) {
	if g == nil {
		return true, nil
	}
	if g.mode == conditionModeAnd {
		for _, cond := range g.conditions {
			ok, err := cond.Eval(ctx, node)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	}
	for _, cond := range g.conditions {
		ok, err := cond.Eval(ctx, node)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func evalTwoStringCondition(name string, args []compiledConditionArg, ctx *evalContext) (bool, error) {
	left, err := args[0].template.Render(ctx)
	if err != nil {
		return false, err
	}
	right, err := args[1].template.Render(ctx)
	if err != nil {
		return false, err
	}
	switch name {
	case "contains":
		return strings.Contains(left, right), nil
	case "equals":
		return left == right, nil
	case "starts_with":
		return strings.HasPrefix(left, right), nil
	case "ends_with":
		return strings.HasSuffix(left, right), nil
	default:
		return false, fmt.Errorf("unknown condition: %s: %w", name, errUnknownCondition)
	}
}

func evalSelectorExists(args []compiledConditionArg, ctx *evalContext, node *html.Node) (bool, error) {
	if node == nil {
		if strings.TrimSpace(ctx.body) == "" {
			return false, nil
		}
		parsed, err := htmlquery.Parse(bytes.NewReader([]byte(ctx.body)))
		if err != nil {
			return false, fmt.Errorf("parse html for selector_exists: %w", err)
		}
		node = parsed
	}
	return htmlquery.FindOne(node, args[0].selector.expr) != nil, nil
}

func (c *compiledCondition) Eval(ctx *evalContext, node *html.Node) (bool, error) {
	switch c.name {
	case "contains", "equals", "starts_with", "ends_with":
		return evalTwoStringCondition(c.name, c.args, ctx)
	case "regex_match":
		left, err := c.args[0].template.Render(ctx)
		if err != nil {
			return false, err
		}
		return c.args[1].regex.MatchString(left), nil
	case "selector_exists":
		return evalSelectorExists(c.args, ctx, node)
	default:
		return false, fmt.Errorf("unknown condition: %s: %w", c.name, errUnknownCondition)
	}
}
