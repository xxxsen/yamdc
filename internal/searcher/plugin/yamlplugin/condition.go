package yamlplugin

import (
	"bytes"
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

type compiledConditionGroup struct {
	mode       conditionMode
	conditions []*compiledCondition
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
		return nil, nil
	}
	mode := conditionMode(spec.Mode)
	if mode != conditionModeAnd && mode != conditionModeOr {
		return nil, fmt.Errorf("invalid condition mode:%s", spec.Mode)
	}
	if len(spec.Conditions) == 0 {
		return nil, fmt.Errorf("conditions is empty")
	}
	out := &compiledConditionGroup{mode: mode, conditions: make([]*compiledCondition, 0, len(spec.Conditions))}
	for _, raw := range spec.Conditions {
		c, err := compileCondition(raw)
		if err != nil {
			return nil, err
		}
		out.conditions = append(out.conditions, c)
	}
	return out, nil
}

func compileCondition(raw string) (*compiledCondition, error) {
	name, args, ok, err := parseCall(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("invalid condition:%s", raw)
	}
	out := &compiledCondition{name: name}
	switch name {
	case "contains", "equals", "starts_with", "ends_with":
		if len(args) != 2 {
			return nil, fmt.Errorf("%s expects 2 arguments", name)
		}
		for _, arg := range args {
			t, ok, err := compileQuotedStringArg(arg)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("%s expects string arguments", name)
			}
			out.args = append(out.args, compiledConditionArg{kind: "string", template: t})
		}
	case "regex_match":
		if len(args) != 2 {
			return nil, fmt.Errorf("regex_match expects 2 arguments")
		}
		left, ok, err := compileQuotedStringArg(args[0])
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("regex_match expects string first argument")
		}
		pattern, ok, err := unquoteArg(strings.TrimSpace(args[1]))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("regex_match expects quoted regex pattern")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compile regex failed, err:%w", err)
		}
		out.args = append(out.args, compiledConditionArg{kind: "string", template: left})
		out.args = append(out.args, compiledConditionArg{kind: "regex", regex: re})
	case "selector_exists":
		if len(args) != 1 {
			return nil, fmt.Errorf("selector_exists expects 1 argument")
		}
		sel, err := compileSelectorLiteral(args[0])
		if err != nil {
			return nil, err
		}
		out.args = append(out.args, compiledConditionArg{kind: "selector", selector: sel})
	default:
		return nil, fmt.Errorf("unknown condition function:%s", name)
	}
	return out, nil
}

func compileQuotedStringArg(raw string) (*template, bool, error) {
	value, ok, err := unquoteArg(strings.TrimSpace(raw))
	if err != nil || !ok {
		return nil, ok, err
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
		return nil, fmt.Errorf("selector_exists expects xpath(\"...\")")
	}
	expr, ok, err := unquoteArg(strings.TrimSpace(args[0]))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("xpath expects quoted string")
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

func (c *compiledCondition) Eval(ctx *evalContext, node *html.Node) (bool, error) {
	switch c.name {
	case "contains":
		left, err := c.args[0].template.Render(ctx)
		if err != nil {
			return false, err
		}
		right, err := c.args[1].template.Render(ctx)
		if err != nil {
			return false, err
		}
		return strings.Contains(left, right), nil
	case "equals":
		left, err := c.args[0].template.Render(ctx)
		if err != nil {
			return false, err
		}
		right, err := c.args[1].template.Render(ctx)
		if err != nil {
			return false, err
		}
		return left == right, nil
	case "starts_with":
		left, err := c.args[0].template.Render(ctx)
		if err != nil {
			return false, err
		}
		right, err := c.args[1].template.Render(ctx)
		if err != nil {
			return false, err
		}
		return strings.HasPrefix(left, right), nil
	case "ends_with":
		left, err := c.args[0].template.Render(ctx)
		if err != nil {
			return false, err
		}
		right, err := c.args[1].template.Render(ctx)
		if err != nil {
			return false, err
		}
		return strings.HasSuffix(left, right), nil
	case "regex_match":
		left, err := c.args[0].template.Render(ctx)
		if err != nil {
			return false, err
		}
		return c.args[1].regex.MatchString(left), nil
	case "selector_exists":
		if node == nil {
			if strings.TrimSpace(ctx.body) == "" {
				return false, nil
			}
			parsed, err := htmlquery.Parse(bytes.NewReader([]byte(ctx.body)))
			if err != nil {
				return false, err
			}
			node = parsed
		}
		return htmlquery.FindOne(node, c.args[0].selector.expr) != nil, nil
	default:
		return false, fmt.Errorf("unknown condition:%s", c.name)
	}
}

