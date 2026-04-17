package yaml

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"golang.org/x/net/html"
)

func (p *SearchPlugin) decodeHTML(ctx context.Context, node *html.Node) (*model.MovieMeta, error) {
	mv := &model.MovieMeta{
		Cover:  &model.File{},
		Poster: &model.File{},
	}
	for _, field := range p.spec.scrape.fields {
		switch field.name {
		case "actors", "genres", "sample_images":
			values := decoder.DecodeList(node, field.selector.expr)
			values = applyListTransforms(values, field.transforms)
			if field.required && len(values) == 0 {
				return nil, nil //nolint:nilnil // nil signals "not found" to caller
			}
			if err := assignListField(ctx, mv, field.name, values, field.parser); err != nil {
				return nil, err
			}
		default:
			value := decoder.DecodeSingle(node, field.selector.expr)
			value = applyStringTransforms(value, field.transforms)
			if field.required && strings.TrimSpace(value) == "" {
				return nil, nil //nolint:nilnil // nil signals "not found" to caller
			}
			if err := assignStringField(ctx, mv, field.name, value, field.parser); err != nil {
				return nil, err
			}
		}
	}
	return mv, nil
}

func (p *SearchPlugin) decodeJSON(ctx context.Context, data []byte) (*model.MovieMeta, error) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode json data failed, err:%w", err)
	}
	mv := &model.MovieMeta{
		Cover:  &model.File{},
		Poster: &model.File{},
	}
	for _, field := range p.spec.scrape.fields {
		values, err := evalJSONPathStrings(doc, field.selector.expr)
		if err != nil {
			return nil, err
		}
		switch field.name {
		case "actors", "genres", "sample_images":
			values = applyListTransforms(values, field.transforms)
			if field.required && len(values) == 0 {
				return nil, nil //nolint:nilnil // nil signals "not found" to caller
			}
			if err := assignListField(ctx, mv, field.name, values, field.parser); err != nil {
				return nil, err
			}
		default:
			value := ""
			if len(values) > 0 {
				value = values[0]
			}
			value = applyStringTransforms(value, field.transforms)
			if field.required && strings.TrimSpace(value) == "" {
				return nil, nil //nolint:nilnil // nil signals "not found" to caller
			}
			if err := assignStringField(ctx, mv, field.name, value, field.parser); err != nil {
				return nil, err
			}
		}
	}
	return mv, nil
}

func assignStringFieldByName(mv *model.MovieMeta, field, value string) {
	switch field {
	case "number":
		mv.Number = value
	case "title":
		mv.Title = value
	case "plot":
		mv.Plot = value
	case "studio":
		mv.Studio = value
	case "label":
		mv.Label = value
	case "director":
		mv.Director = value
	case "series":
		mv.Series = value
	case "cover":
		mv.Cover.Name = value
	case "poster":
		mv.Poster.Name = value
	}
}

func parseDurationMMSS(value string) int64 {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0
	}
	minutes, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0
	}
	return minutes*60 + sec
}

func parseDurationByKind(ctx context.Context, kind, value string) int64 {
	switch kind {
	case "duration_default":
		return parser.DefaultDurationParser(ctx)(value)
	case "duration_hhmmss":
		return parser.DefaultHHMMSSDurationParser(ctx)(value)
	case "duration_mm":
		return parser.DefaultMMDurationParser(ctx)(value)
	case "duration_human":
		return parser.HumanDurationToSecond(value)
	case "duration_mmss":
		return parseDurationMMSS(value)
	default:
		return 0
	}
}

func assignDateField(mv *model.MovieMeta, field string, parserSpec ParserSpec, value string) error {
	switch parserSpec.Kind {
	case "time_format":
		t, err := timeParse(parserSpec.Layout, value)
		if err != nil {
			return err
		}
		if field == "release_date" {
			mv.ReleaseDate = t
		}
	case "date_layout_soft":
		if field == "release_date" {
			mv.ReleaseDate = softTimeParse(parserSpec.Layout, value)
		}
	}
	return nil
}

func assignStringField(ctx context.Context, mv *model.MovieMeta, field, value string, parserSpec ParserSpec) error {
	switch parserSpec.Kind {
	case "", "string":
		assignStringFieldByName(mv, field, value)
	case "date_only":
		mv.ReleaseDate = parser.DateOnlyReleaseDateParser(ctx)(value)
	case "duration_default", "duration_hhmmss", "duration_mm", "duration_human", "duration_mmss":
		mv.Duration = parseDurationByKind(ctx, parserSpec.Kind, value)
	case "time_format", "date_layout_soft":
		return assignDateField(mv, field, parserSpec, value)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedParser, parserSpec.Kind)
	}
	return nil
}

func assignListField(
	_ context.Context, mv *model.MovieMeta, field string, values []string, parserSpec ParserSpec,
) error {
	switch parserSpec.Kind {
	case "", "string_list":
		switch field {
		case "actors":
			mv.Actors = values
		case "genres":
			mv.Genres = values
		case "sample_images":
			for _, item := range values {
				mv.SampleImages = append(mv.SampleImages, &model.File{Name: item})
			}
		}
	default:
		return fmt.Errorf("%w: %s", errUnsupportedListParser, parserSpec.Kind)
	}
	return nil
}

func applyStringTransforms(value string, transforms []*TransformSpec) string {
	out := value
	for _, item := range transforms {
		switch item.Kind {
		case "trim":
			out = strings.TrimSpace(out)
		case "trim_prefix":
			out = strings.TrimPrefix(out, item.Value)
		case "trim_suffix":
			out = strings.TrimSuffix(out, item.Value)
		case "trim_charset":
			out = strings.Trim(out, item.Cutset)
		case "replace":
			out = strings.ReplaceAll(out, item.Old, item.New)
		case "regex_extract":
			re, err := regexp.Compile(item.Value)
			if err != nil {
				out = ""
				continue
			}
			matches := re.FindStringSubmatch(out)
			if item.Index >= 0 && item.Index < len(matches) {
				out = matches[item.Index]
			} else {
				out = ""
			}
		case "split_index":
			parts := strings.Split(out, item.Sep)
			if item.Index >= 0 && item.Index < len(parts) {
				out = parts[item.Index]
			} else {
				out = ""
			}
		case "to_upper":
			out = strings.ToUpper(out)
		case "to_lower":
			out = strings.ToLower(out)
		}
	}
	return out
}

func applyOneListTransform(out []string, item *TransformSpec) []string {
	switch item.Kind {
	case "remove_empty":
		filtered := make([]string, 0, len(out))
		for _, value := range out {
			if strings.TrimSpace(value) != "" {
				filtered = append(filtered, value)
			}
		}
		return filtered
	case "dedupe":
		seen := make(map[string]struct{}, len(out))
		deduped := make([]string, 0, len(out))
		for _, value := range out {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			deduped = append(deduped, value)
		}
		return deduped
	case "map_trim":
		for i, value := range out {
			out[i] = strings.TrimSpace(value)
		}
	case "replace":
		for i, value := range out {
			out[i] = strings.ReplaceAll(value, item.Old, item.New)
		}
	case "split":
		split := make([]string, 0, len(out))
		for _, value := range out {
			split = append(split, strings.Split(value, item.Sep)...)
		}
		return split
	case "to_upper":
		for i, value := range out {
			out[i] = strings.ToUpper(value)
		}
	case "to_lower":
		for i, value := range out {
			out[i] = strings.ToLower(value)
		}
	}
	return out
}

func applyListTransforms(values []string, transforms []*TransformSpec) []string {
	out := append([]string(nil), values...)
	for _, item := range transforms {
		out = applyOneListTransform(out, item)
	}
	return out
}

func (p *SearchPlugin) applyPostprocess(ctx context.Context, mv *model.MovieMeta) {
	if p.spec.postprocess == nil {
		return
	}
	metaMap := movieMetaStringMap(mv)
	if len(p.spec.postprocess.assign) != 0 {
		evalCtx := &evalContext{
			number: ctxNumber(ctx),
			host:   currentHost(ctx, p.spec.hosts),
			vars:   readVarsFromContext(ctx),
			meta:   metaMap,
		}
		for key, tmpl := range p.spec.postprocess.assign {
			value, err := tmpl.Render(evalCtx)
			if err != nil {
				continue
			}
			_ = assignStringField(ctx, mv, key, value, ParserSpec{Kind: "string"})
			metaMap = movieMetaStringMap(mv)
			evalCtx.meta = metaMap
		}
	}
	if p.spec.postprocess.defaults != nil {
		mv.TitleLang = normalizeLang(p.spec.postprocess.defaults.TitleLang)
		mv.PlotLang = normalizeLang(p.spec.postprocess.defaults.PlotLang)
		mv.GenresLang = normalizeLang(p.spec.postprocess.defaults.GenresLang)
		mv.ActorsLang = normalizeLang(p.spec.postprocess.defaults.ActorsLang)
	}
	if p.spec.postprocess.switchConfig != nil {
		mv.SwithConfig.DisableReleaseDateCheck = p.spec.postprocess.switchConfig.DisableReleaseDateCheck
		mv.SwithConfig.DisableNumberReplace = p.spec.postprocess.switchConfig.DisableNumberReplace
	}
}
