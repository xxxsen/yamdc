package yamlplugin

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type jsonPathTokenKind string

const (
	jsonPathField jsonPathTokenKind = "field"
	jsonPathIndex jsonPathTokenKind = "index"
	jsonPathWild  jsonPathTokenKind = "wildcard"
)

type jsonPathToken struct {
	kind  jsonPathTokenKind
	field string
	index int
}

func evalJSONPathStrings(doc any, expr string) ([]string, error) {
	values, err := evalJSONPath(doc, expr)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		switch v := value.(type) {
		case nil:
			continue
		case string:
			out = append(out, v)
		case float64:
			out = append(out, strconv.FormatFloat(v, 'f', -1, 64))
		case bool:
			out = append(out, strconv.FormatBool(v))
		default:
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			out = append(out, string(raw))
		}
	}
	return out, nil
}

func evalJSONPath(doc any, expr string) ([]any, error) {
	tokens, err := parseJSONPath(expr)
	if err != nil {
		return nil, err
	}
	current := []any{doc}
	for _, token := range tokens {
		next := make([]any, 0)
		for _, item := range current {
			switch token.kind {
			case jsonPathField:
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				value, ok := obj[token.field]
				if !ok {
					continue
				}
				next = append(next, value)
			case jsonPathIndex:
				list, ok := item.([]any)
				if !ok {
					continue
				}
				if token.index < 0 || token.index >= len(list) {
					continue
				}
				next = append(next, list[token.index])
			case jsonPathWild:
				list, ok := item.([]any)
				if !ok {
					continue
				}
				next = append(next, list...)
			default:
				return nil, fmt.Errorf("unsupported jsonpath token kind:%s", token.kind)
			}
		}
		current = next
	}
	return current, nil
}

func parseJSONPath(expr string) ([]jsonPathToken, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("jsonpath is empty")
	}
	if expr == "$" {
		return nil, nil
	}
	if !strings.HasPrefix(expr, "$.") {
		return nil, fmt.Errorf("jsonpath must start with $.")
	}
	expr = expr[2:]
	tokens := make([]jsonPathToken, 0)
	for expr != "" {
		fieldEnd := strings.IndexAny(expr, ".[")
		field := expr
		if fieldEnd >= 0 {
			field = expr[:fieldEnd]
		}
		if field != "" {
			tokens = append(tokens, jsonPathToken{kind: jsonPathField, field: field})
			expr = expr[len(field):]
		}
		for strings.HasPrefix(expr, "[") {
			end := strings.IndexByte(expr, ']')
			if end < 0 {
				return nil, fmt.Errorf("invalid jsonpath:%s", expr)
			}
			content := expr[1:end]
			switch content {
			case "*":
				tokens = append(tokens, jsonPathToken{kind: jsonPathWild})
			default:
				idx, err := strconv.Atoi(content)
				if err != nil {
					return nil, fmt.Errorf("invalid jsonpath index:%s", content)
				}
				tokens = append(tokens, jsonPathToken{kind: jsonPathIndex, index: idx})
			}
			expr = expr[end+1:]
		}
		if strings.HasPrefix(expr, ".") {
			expr = expr[1:]
			continue
		}
		if expr != "" {
			return nil, fmt.Errorf("invalid jsonpath expression near:%s", expr)
		}
	}
	return tokens, nil
}
