package yaml

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/net/html"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
)

func (p *SearchPlugin) traceDecodeHTML(ctx context.Context, node *html.Node, out map[string]FieldDebugResult) (
	*model.MovieMeta,
	error,
) {
	mv := &model.MovieMeta{
		Cover:  &model.File{},
		Poster: &model.File{},
	}
	fieldNames := make([]string, 0, len(p.spec.scrape.fields))
	for _, field := range p.spec.scrape.fields {
		fieldNames = append(fieldNames, field.name)
	}
	sort.Strings(fieldNames)
	for _, fieldName := range fieldNames {
		field := p.fieldByName(fieldName)
		dbg, err := p.traceFieldHTML(ctx, mv, node, field)
		if err != nil {
			return nil, err
		}
		out[field.name] = dbg
		if field.required && !dbg.Matched {
			return nil, nil //nolint:nilnil // nil signals "not found" to caller
		}
	}
	return mv, nil
}

func (p *SearchPlugin) traceDecodeJSON(ctx context.Context, data []byte, out map[string]FieldDebugResult) (
	*model.MovieMeta,
	error,
) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode json data failed, err:%w", err)
	}
	mv := &model.MovieMeta{
		Cover:  &model.File{},
		Poster: &model.File{},
	}
	fieldNames := make([]string, 0, len(p.spec.scrape.fields))
	for _, field := range p.spec.scrape.fields {
		fieldNames = append(fieldNames, field.name)
	}
	sort.Strings(fieldNames)
	for _, fieldName := range fieldNames {
		field := p.fieldByName(fieldName)
		dbg, err := p.traceFieldJSON(ctx, mv, doc, field)
		if err != nil {
			return nil, err
		}
		out[field.name] = dbg
		if field.required && !dbg.Matched {
			return nil, nil //nolint:nilnil // nil signals "not found" to caller
		}
	}
	return mv, nil
}

func (p *SearchPlugin) fieldByName(name string) *compiledField {
	for _, field := range p.spec.scrape.fields {
		if field.name == name {
			return field
		}
	}
	return nil
}

func (p *SearchPlugin) traceFieldHTML(ctx context.Context, mv *model.MovieMeta, node *html.Node, field *compiledField) (
	FieldDebugResult,
	error,
) {
	if isListField(field.name) {
		values := decoder.DecodeList(node, field.selector.expr)
		steps := make([]TransformStep, 0, len(field.transforms))
		out := traceListTransforms(values, field.transforms, &steps)
		dbg := FieldDebugResult{
			SelectorValues: ensureStringSlice(values),
			TransformSteps: steps,
			Required:       field.required,
			Matched:        len(out) > 0,
			ParserResult:   append([]string{}, out...),
		}
		if len(out) > 0 {
			if err := assignListField(ctx, mv, field.name, out, field.parser); err != nil {
				return dbg, err
			}
		}
		return dbg, nil
	}
	value := decoder.DecodeSingle(node, field.selector.expr)
	steps := make([]TransformStep, 0, len(field.transforms))
	out := traceStringTransforms(value, field.transforms, &steps)
	dbg := FieldDebugResult{
		SelectorValues: []string{value},
		TransformSteps: steps,
		Required:       field.required,
		Matched:        strings.TrimSpace(out) != "",
	}
	parserResult, err := traceAssignStringField(ctx, mv, field.name, out, field.parser)
	dbg.ParserResult = parserResult
	return dbg, err
}

func (p *SearchPlugin) traceFieldJSON(ctx context.Context, mv *model.MovieMeta, doc any, field *compiledField) (
	FieldDebugResult,
	error,
) {
	values, err := evalJSONPathStrings(doc, field.selector.expr)
	if err != nil {
		return FieldDebugResult{SelectorValues: []string{}, TransformSteps: []TransformStep{}}, err
	}
	if isListField(field.name) {
		steps := make([]TransformStep, 0, len(field.transforms))
		out := traceListTransforms(values, field.transforms, &steps)
		dbg := FieldDebugResult{
			SelectorValues: ensureStringSlice(values),
			TransformSteps: steps,
			Required:       field.required,
			Matched:        len(out) > 0,
			ParserResult:   append([]string{}, out...),
		}
		if len(out) > 0 {
			if err := assignListField(ctx, mv, field.name, out, field.parser); err != nil {
				return dbg, err
			}
		}
		return dbg, nil
	}
	value := ""
	if len(values) > 0 {
		value = values[0]
	}
	steps := make([]TransformStep, 0, len(field.transforms))
	out := traceStringTransforms(value, field.transforms, &steps)
	dbg := FieldDebugResult{
		SelectorValues: ensureStringSlice(values),
		TransformSteps: steps,
		Required:       field.required,
		Matched:        strings.TrimSpace(out) != "",
	}
	parserResult, err := traceAssignStringField(ctx, mv, field.name, out, field.parser)
	dbg.ParserResult = parserResult
	return dbg, err
}

// ensureStringSlice 确保切片非 nil, 防止 encoding/json 把 nil 序列化成 null
// 导致前端在 .length / .map 直接崩溃。
func ensureStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func isListField(field string) bool {
	switch field {
	case "actors", "genres", "sample_images":
		return true
	default:
		return false
	}
}

func traceStringTransforms(value string, transforms []*TransformSpec, steps *[]TransformStep) string {
	out := value
	for _, item := range transforms {
		input := out
		out = applyStringTransforms(out, []*TransformSpec{item})
		*steps = append(*steps, TransformStep{
			Kind:   item.Kind,
			Input:  input,
			Output: out,
		})
	}
	return out
}

func traceListTransforms(values []string, transforms []*TransformSpec, steps *[]TransformStep) []string {
	out := append([]string(nil), values...)
	for _, item := range transforms {
		input := append([]string(nil), out...)
		out = applyListTransforms(out, []*TransformSpec{item})
		*steps = append(*steps, TransformStep{
			Kind:   item.Kind,
			Input:  input,
			Output: append([]string(nil), out...),
		})
	}
	return out
}

func traceAssignStringField(
	ctx context.Context,
	mv *model.MovieMeta,
	field,
	value string,
	parserSpec ParserSpec,
) (any, error) {
	if strings.TrimSpace(value) == "" && (parserSpec.Kind == "" || parserSpec.Kind == "string") {
		return value, nil
	}
	if err := assignStringField(ctx, mv, field, value, parserSpec); err != nil {
		return nil, err
	}
	switch parserSpec.Kind {
	case "", "string":
		return value, nil
	case "date_only", "time_format", "date_layout_soft":
		return mv.ReleaseDate, nil
	case "duration_default", "duration_hhmmss", "duration_mm", "duration_human", "duration_mmss":
		return mv.Duration, nil
	default:
		return value, nil
	}
}
