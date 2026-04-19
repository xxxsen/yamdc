package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/number"
)

var (
	errHandlerIDRequired       = errors.New("handler_id is required")
	errInvalidMode             = errors.New("invalid mode")
	errHandlerInstanceNotFound = errors.New("handler instance not found")
	errMetaNumberRequired      = errors.New("meta.number is required")
)

type DebugHandlerInstance struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DebugRequest struct {
	Mode       string           `json:"mode"`
	HandlerID  string           `json:"handler_id"`
	HandlerIDs []string         `json:"handler_ids"`
	Meta       *model.MovieMeta `json:"meta"`
}

type DebugStep struct {
	HandlerID   string           `json:"handler_id"`
	HandlerName string           `json:"handler_name"`
	BeforeMeta  *model.MovieMeta `json:"before_meta"`
	AfterMeta   *model.MovieMeta `json:"after_meta"`
	Error       string           `json:"error"`
}

type DebugResult struct {
	Mode        string           `json:"mode"`
	HandlerID   string           `json:"handler_id"`
	HandlerName string           `json:"handler_name"`
	NumberID    string           `json:"number_id"`
	Category    string           `json:"category"`
	Unrated     bool             `json:"unrated"`
	BeforeMeta  *model.MovieMeta `json:"before_meta"`
	AfterMeta   *model.MovieMeta `json:"after_meta"`
	Error       string           `json:"error"`
	Steps       []DebugStep      `json:"steps"`
}

type DebugHandlerOption struct {
	Disable bool
	Args    any
}

type Debugger struct {
	deps      appdeps.Runtime
	cleaner   movieidcleaner.Cleaner
	instances []DebugHandlerInstance
	configs   map[string]DebugHandlerOption
}

func NewDebugger(
	deps appdeps.Runtime,
	cleaner movieidcleaner.Cleaner,
	handlers []string,
	configs map[string]DebugHandlerOption,
) *Debugger {
	items := make([]DebugHandlerInstance, 0, len(handlers))
	cfgMap := make(map[string]DebugHandlerOption, len(handlers))
	for _, name := range handlers {
		if name == HDurationFixer {
			continue
		}
		handlerCfg, ok := configs[name]
		if ok && handlerCfg.Disable {
			continue
		}
		items = append(items, DebugHandlerInstance{
			ID:   name,
			Name: name,
		})
		if ok {
			cfgMap[name] = handlerCfg
		}
	}
	return &Debugger{
		deps:      deps,
		cleaner:   cleaner,
		instances: items,
		configs:   cfgMap,
	}
}

func (d *Debugger) Handlers() []DebugHandlerInstance {
	return append([]DebugHandlerInstance(nil), d.instances...)
}

func (d *Debugger) prepareDebugContext(req DebugRequest) (*DebugResult, *model.FileContext, error) {
	metaInput := req.Meta
	if metaInput == nil {
		metaInput = &model.MovieMeta{}
	}
	beforeMeta, err := cloneMovieMeta(metaInput)
	if err != nil {
		return nil, nil, err
	}
	afterMeta, err := cloneMovieMeta(metaInput)
	if err != nil {
		return nil, nil, err
	}
	num, err := d.parseNumber(afterMeta.Number)
	if err != nil {
		return nil, nil, err
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "single"
	}
	result := &DebugResult{
		Mode:       mode,
		NumberID:   num.GetNumberID(),
		Category:   num.GetExternalFieldCategory(),
		Unrated:    num.GetExternalFieldUnrated(),
		BeforeMeta: beforeMeta,
		AfterMeta:  afterMeta,
		// 预先初始化切片, 避免 nil 切片序列化成 null 后前端直接 .length 崩溃。
		// chain 模式可能因 handler_ids 过滤后链路为空而始终走不进 append。
		Steps: []DebugStep{},
	}
	fc := &model.FileContext{Meta: afterMeta, Number: num}
	return result, fc, nil
}

func (d *Debugger) debugSingleHandler(
	ctx context.Context, fc *model.FileContext, handlerID string, result *DebugResult,
) error {
	instance, handlerCfg, err := d.lookupHandler(handlerID)
	if err != nil {
		return err
	}
	result.HandlerID = instance.ID
	result.HandlerName = instance.Name
	step, err := d.runOne(ctx, fc, *instance, handlerCfg)
	if err != nil {
		return err
	}
	result.Steps = []DebugStep{*step}
	result.Error = step.Error
	return nil
}

func (d *Debugger) debugChainHandlers(
	ctx context.Context, fc *model.FileContext, handlerIDs []string, result *DebugResult,
) error {
	chain := d.resolveChain(handlerIDs)
	failCount := 0
	for _, instance := range chain {
		step, err := d.runOne(ctx, fc, instance, d.configs[instance.ID])
		if err != nil {
			return err
		}
		result.Steps = append(result.Steps, *step)
		if step.Error != "" {
			failCount++
		}
	}
	if failCount > 0 {
		result.Error = fmt.Sprintf("%d handlers failed", failCount)
	}
	return nil
}

func (d *Debugger) Debug(ctx context.Context, req DebugRequest) (*DebugResult, error) {
	mode := strings.TrimSpace(req.Mode)
	handlerID := strings.TrimSpace(req.HandlerID)
	if mode == "single" || (mode == "" && handlerID == "") {
		if handlerID == "" && (mode == "" || mode == "single") {
			return nil, errHandlerIDRequired
		}
	}
	debugResult, fc, err := d.prepareDebugContext(req)
	if err != nil {
		return nil, err
	}
	switch debugResult.Mode {
	case "single":
		if err := d.debugSingleHandler(ctx, fc, strings.TrimSpace(req.HandlerID), debugResult); err != nil {
			return nil, err
		}
	case "chain":
		if err := d.debugChainHandlers(ctx, fc, req.HandlerIDs, debugResult); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid mode: %s: %w", debugResult.Mode, errInvalidMode)
	}
	return debugResult, nil
}

func (d *Debugger) resolveChain(handlerIDs []string) []DebugHandlerInstance {
	if len(handlerIDs) == 0 {
		return append([]DebugHandlerInstance(nil), d.instances...)
	}
	allowed := make(map[string]struct{}, len(handlerIDs))
	for _, id := range handlerIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		allowed[id] = struct{}{}
	}
	chain := make([]DebugHandlerInstance, 0, len(allowed))
	for _, instance := range d.instances {
		if _, ok := allowed[instance.ID]; ok {
			chain = append(chain, instance)
		}
	}
	return chain
}

func (d *Debugger) runOne(
	ctx context.Context,
	fc *model.FileContext,
	instance DebugHandlerInstance,
	handlerCfg DebugHandlerOption,
) (*DebugStep, error) {
	beforeMeta, err := cloneMovieMeta(fc.Meta)
	if err != nil {
		return nil, err
	}
	h, err := CreateHandler(instance.Name, handlerCfg.Args, d.deps)
	if err != nil {
		return nil, err
	}
	step := &DebugStep{
		HandlerID:   instance.ID,
		HandlerName: instance.Name,
		BeforeMeta:  beforeMeta,
	}
	if err := h.Handle(ctx, fc); err != nil {
		step.Error = err.Error()
	}
	afterMeta, err := cloneMovieMeta(fc.Meta)
	if err != nil {
		return nil, err
	}
	step.AfterMeta = afterMeta
	return step, nil
}

func (d *Debugger) lookupHandler(handlerID string) (*DebugHandlerInstance, DebugHandlerOption, error) {
	for _, item := range d.instances {
		if item.ID == handlerID {
			return &item, d.configs[handlerID], nil
		}
	}
	return nil, DebugHandlerOption{}, fmt.Errorf(
		"handler instance not found: %s: %w",
		handlerID,
		errHandlerInstanceNotFound,
	)
}

func (d *Debugger) parseNumber(rawInput string) (*number.Number, error) {
	input := strings.TrimSpace(rawInput)
	if input == "" {
		return nil, errMetaNumberRequired
	}
	if d.cleaner != nil {
		num, err := d.tryCleanNumber(input)
		if err != nil {
			return nil, err
		}
		if num != nil {
			return num, nil
		}
	}
	num, err := number.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("parse number failed: %w", err)
	}
	return num, nil
}

func (d *Debugger) tryCleanNumber(input string) (*number.Number, error) {
	res, err := d.cleaner.Clean(input)
	if err != nil {
		return nil, cleanNumberError(err)
	}
	return buildNumberFromCleanResult(res)
}

func cleanNumberError(err error) error {
	var cleanErr *movieidcleaner.CleanError
	if errors.As(err, &cleanErr) {
		return nil
	}
	return fmt.Errorf("clean number failed: %w", err)
}

func buildNumberFromCleanResult(res *movieidcleaner.Result) (*number.Number, error) {
	if res == nil || strings.TrimSpace(res.NumberID) == "" {
		return nil, nil //nolint:nilnil // nil signals fallback to raw parse
	}
	num, parseErr := number.Parse(res.NumberID)
	if parseErr != nil {
		return nil, nil //nolint:nilnil,nilerr // nil signals fallback to raw parse
	}
	if res.Category != "" {
		num.SetExternalFieldCategory(res.Category)
	}
	if res.Unrated {
		num.SetExternalFieldUnrated(true)
	}
	return num, nil
}

func cloneMovieMeta(in *model.MovieMeta) (*model.MovieMeta, error) {
	if in == nil {
		return &model.MovieMeta{}, nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("marshal movie meta failed: %w", err)
	}
	var out model.MovieMeta
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal movie meta failed: %w", err)
	}
	return &out, nil
}
