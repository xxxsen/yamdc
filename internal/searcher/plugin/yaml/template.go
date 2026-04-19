package yaml

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

var (
	errUnknownTemplateFunction   = errors.New("unknown template function")
	errInvalidTemplateExpr       = errors.New("invalid template expression")
	errUnterminatedTemplate      = errors.New("unterminated template")
	errBuildURLExpects2Args      = errors.New("build_url expects 2 arguments")
	errToUpperExpects1Arg        = errors.New("to_upper expects 1 argument")
	errToLowerExpects1Arg        = errors.New("to_lower expects 1 argument")
	errTrimExpects1Arg           = errors.New("trim expects 1 argument")
	errTrimPrefixExpects2Args    = errors.New("trim_prefix expects 2 arguments")
	errTrimSuffixExpects2Args    = errors.New("trim_suffix expects 2 arguments")
	errReplaceExpects3Args       = errors.New("replace expects 3 arguments")
	errCleanNumberExpects1Arg    = errors.New("clean_number expects 1 argument")
	errFirstNonEmptyExpects2Args = errors.New("first_non_empty expects at least 2 arguments")
	errLastSegmentExpects2Args   = errors.New("last_segment expects 2 arguments")
	errUnknownTemplateVariable   = errors.New("unknown template variable")
	errInvalidArgumentList       = errors.New("invalid argument list")
)

type evalContext struct {
	number        string
	host          string
	body          string
	vars          map[string]string
	item          map[string]string
	itemVariables map[string]string
	meta          map[string]string
	value         string
	candidate     string
}

type template struct {
	raw string
}

func compileTemplate(raw string) (*template, error) {
	t := &template{raw: raw}
	if err := validateTemplate(raw); err != nil {
		return nil, err
	}
	return t, nil
}

func validateTemplate(raw string) error {
	for i := 0; i < len(raw); {
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			end, err := findTemplateEnd(raw, i)
			if err != nil {
				return err
			}
			if err := validateTemplateExpr(raw[i+2 : end]); err != nil {
				return err
			}
			i = end + 1
			continue
		}
		i++
	}
	return nil
}

func validateTemplateExpr(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	if name, args, ok, err := parseCall(expr); err != nil {
		return err
	} else if ok {
		knownFuncs := []string{
			"build_url", "to_upper", "to_lower", "trim", "trim_prefix",
			"trim_suffix", "replace", "clean_number", "first_non_empty",
			"concat", "last_segment",
		}
		if !slices.Contains(knownFuncs, name) {
			return fmt.Errorf("unknown template function: %s: %w", name, errUnknownTemplateFunction)
		}
		for _, arg := range args {
			if err := validateTemplateArg(arg); err != nil {
				return err
			}
		}
		return nil
	}
	if isVariableRef(expr) {
		return nil
	}
	return fmt.Errorf("invalid template expression: %s: %w", expr, errInvalidTemplateExpr)
}

func validateTemplateArg(arg string) error {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil
	}
	if quoted, ok := unquoteArg(arg); ok {
		return validateTemplate(quoted)
	}
	if strings.HasPrefix(arg, "${") && strings.HasSuffix(arg, "}") {
		return validateTemplate(arg)
	}
	if strings.Contains(arg, "${") {
		return validateTemplate(arg)
	}
	if _, _, ok, err := parseCall(arg); err != nil {
		return err
	} else if ok {
		return validateTemplateExpr(arg)
	}
	if isVariableRef(arg) {
		return nil
	}
	return nil
}

func (t *template) Render(ctx *evalContext) (string, error) {
	return renderTemplate(t.raw, ctx)
}

func renderTemplate(raw string, ctx *evalContext) (string, error) {
	if !strings.Contains(raw, "${") {
		return raw, nil
	}
	var sb strings.Builder
	for i := 0; i < len(raw); {
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			end, err := findTemplateEnd(raw, i)
			if err != nil {
				return "", err
			}
			expr := raw[i+2 : end]
			value, err := evalTemplateExpr(expr, ctx)
			if err != nil {
				return "", err
			}
			sb.WriteString(value)
			i = end + 1
			continue
		}
		sb.WriteByte(raw[i])
		i++
	}
	return sb.String(), nil
}

func findTemplateEnd(raw string, start int) (int, error) {
	depth := 0
	for i := start; i < len(raw); i++ {
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			depth++
			i++
			continue
		}
		if raw[i] == '}' {
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated template: %s: %w", raw[start:], errUnterminatedTemplate)
}

func evalTemplateExpr(expr string, ctx *evalContext) (string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", nil
	}
	if name, args, ok, err := parseCall(expr); err != nil {
		return "", err
	} else if ok {
		values := make([]string, 0, len(args))
		for _, arg := range args {
			v, err := evalTemplateArg(arg, ctx)
			if err != nil {
				return "", err
			}
			values = append(values, v)
		}
		return evalTemplateFunc(name, values)
	}
	return resolveTemplateVar(expr, ctx)
}

func evalTemplateArg(arg string, ctx *evalContext) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", nil
	}
	if quoted, ok := unquoteArg(arg); ok {
		return renderTemplate(quoted, ctx)
	}
	if strings.HasPrefix(arg, "${") && strings.HasSuffix(arg, "}") {
		return renderTemplate(arg, ctx)
	}
	if strings.Contains(arg, "${") {
		return renderTemplate(arg, ctx)
	}
	if name, args, ok, err := parseCall(arg); err != nil {
		return "", err
	} else if ok {
		values := make([]string, 0, len(args))
		for _, item := range args {
			v, err := evalTemplateArg(item, ctx)
			if err != nil {
				return "", err
			}
			values = append(values, v)
		}
		return evalTemplateFunc(name, values)
	}
	if isVariableRef(arg) {
		return resolveTemplateVar(arg, ctx)
	}
	return arg, nil
}

func evalBuildURL(args []string) (string, error) {
	if len(args) != 2 {
		return "", errBuildURLExpects2Args
	}
	if u, err := url.Parse(args[1]); err == nil && u.IsAbs() {
		return args[1], nil
	}
	base, err := url.Parse(args[0])
	if err != nil {
		return "", fmt.Errorf("parse base url failed, err:%w", err)
	}
	ref, err := url.Parse(args[1])
	if err != nil {
		return "", fmt.Errorf("parse ref url failed, err:%w", err)
	}
	return base.ResolveReference(ref).String(), nil
}

func evalLastSegment(args []string) (string, error) {
	if len(args) != 2 {
		return "", errLastSegmentExpects2Args
	}
	if args[1] == "" {
		return args[0], nil
	}
	parts := strings.Split(args[0], args[1])
	if len(parts) == 0 {
		return "", nil
	}
	return parts[len(parts)-1], nil
}

func evalFirstNonEmpty(args []string) (string, error) {
	if len(args) < 2 {
		return "", errFirstNonEmptyExpects2Args
	}
	for _, item := range args {
		if strings.TrimSpace(item) != "" {
			return item, nil
		}
	}
	return "", nil
}

// evalTemplateFunc 按 name 分派内建模板函数, 每个 case 含一次参数个数校验 + 一次
// 实现调用. 改表驱动需要把 "函数名 -> 期望 arg 数 -> 实现" 三张表配齐, 对于
// 仅十来个条目的 switch 而言, 可读性反而更差.
//
//nolint:gocyclo // builtin template function dispatch table
func evalTemplateFunc(name string, args []string) (string, error) {
	switch name {
	case "build_url":
		return evalBuildURL(args)
	case "to_upper":
		if len(args) != 1 {
			return "", errToUpperExpects1Arg
		}
		return strings.ToUpper(args[0]), nil
	case "to_lower":
		if len(args) != 1 {
			return "", errToLowerExpects1Arg
		}
		return strings.ToLower(args[0]), nil
	case "trim":
		if len(args) != 1 {
			return "", errTrimExpects1Arg
		}
		return strings.TrimSpace(args[0]), nil
	case "trim_prefix":
		if len(args) != 2 {
			return "", errTrimPrefixExpects2Args
		}
		return strings.TrimPrefix(args[0], args[1]), nil
	case "trim_suffix":
		if len(args) != 2 {
			return "", errTrimSuffixExpects2Args
		}
		return strings.TrimSuffix(args[0], args[1]), nil
	case "replace":
		if len(args) != 3 {
			return "", errReplaceExpects3Args
		}
		return strings.ReplaceAll(args[0], args[1], args[2]), nil
	case "clean_number":
		if len(args) != 1 {
			return "", errCleanNumberExpects1Arg
		}
		return strings.NewReplacer("-", "", "_", "").Replace(args[0]), nil
	case "first_non_empty":
		return evalFirstNonEmpty(args)
	case "concat":
		return strings.Join(args, ""), nil
	case "last_segment":
		return evalLastSegment(args)
	default:
		return "", fmt.Errorf("unknown template function: %s: %w", name, errUnknownTemplateFunction)
	}
}

func resolveTemplateVar(ref string, ctx *evalContext) (string, error) {
	switch ref {
	case "number":
		return ctx.number, nil
	case "host":
		return ctx.host, nil
	case "body":
		return ctx.body, nil
	case "value":
		return ctx.value, nil
	case "candidate":
		return ctx.candidate, nil
	}
	if v, ok := resolveMapRef(ref, "vars.", ctx.vars); ok {
		return v, nil
	}
	if v, ok := resolveMapRef(ref, "item.", ctx.item); ok {
		return v, nil
	}
	if v, ok := resolveMapRef(ref, "item_variables.", ctx.itemVariables); ok {
		return v, nil
	}
	if v, ok := resolveMapRef(ref, "meta.", ctx.meta); ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown template variable: %s: %w", ref, errUnknownTemplateVariable)
}

func resolveMapRef(ref, prefix string, m map[string]string) (string, bool) {
	if !strings.HasPrefix(ref, prefix) {
		return "", false
	}
	if m == nil {
		return "", false
	}
	v, ok := m[strings.TrimPrefix(ref, prefix)]
	return v, ok
}

func parseCall(in string) (string, []string, bool, error) {
	in = strings.TrimSpace(in)
	open := strings.IndexByte(in, '(')
	if open <= 0 || !strings.HasSuffix(in, ")") {
		return "", nil, false, nil
	}
	name := strings.TrimSpace(in[:open])
	if !isIdentifier(name) {
		return "", nil, false, nil
	}
	args, err := splitArgs(in[open+1 : len(in)-1])
	if err != nil {
		return "", nil, false, err
	}
	return name, args, true, nil
}

func splitArgs(in string) ([]string, error) {
	if strings.TrimSpace(in) == "" {
		return nil, nil
	}
	var args []string
	start := 0
	depthParen := 0
	inQuote := false
	escaped := false
	for i := 0; i < len(in); i++ {
		ch := in[i]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			continue
		}
		switch ch {
		case '"':
			inQuote = true
		case '(':
			depthParen++
		case ')':
			depthParen--
			if depthParen < 0 {
				return nil, fmt.Errorf("invalid argument list: %s: %w", in, errInvalidArgumentList)
			}
		case ',':
			if depthParen == 0 {
				args = append(args, strings.TrimSpace(in[start:i]))
				start = i + 1
			}
		}
	}
	if inQuote || depthParen != 0 {
		return nil, fmt.Errorf("invalid argument list: %s: %w", in, errInvalidArgumentList)
	}
	args = append(args, strings.TrimSpace(in[start:]))
	return args, nil
}

func unquoteArg(in string) (string, bool) {
	if len(in) < 2 || in[0] != '"' || in[len(in)-1] != '"' {
		return "", false
	}
	return strconvUnquote(in), true
}

func strconvUnquote(in string) string {
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(in[1 : len(in)-1])
}

// isIdentifier 按首字符 / 后续字符的 Unicode 类别做分支校验, 每个 branch 对应
// 一类合法字符 (字母 / 数字 / underscore / 特定符号). 拆成"首字符检查函数"+
// "后续字符检查函数"会把关联性强的规则割裂, 反而不直观.
//
//nolint:gocyclo // per-rune unicode class dispatch
func isIdentifier(in string) bool {
	if in == "" {
		return false
	}
	for i, ch := range in {
		if i == 0 {
			if ch != '_' && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') {
				return false
			}
			continue
		}
		if ch != '_' && ch != '.' && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}

func isVariableRef(in string) bool {
	if !isIdentifier(in) {
		return false
	}
	switch {
	case in == "number", in == "host", in == "body", in == "value", in == "candidate":
		return true
	case strings.HasPrefix(in, "vars."), strings.HasPrefix(in, "item."), strings.HasPrefix(
		in,
		"item_variables.",
	), strings.HasPrefix(in, "meta."):
		return true
	default:
		return false
	}
}

func selectedHost(ctx *evalContext, hosts []string) string {
	if ctx != nil && ctx.host != "" {
		return ctx.host
	}
	if len(hosts) == 0 {
		return ""
	}
	return pluginapi.MustSelectDomain(hosts)
}
