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
	Uncensor    bool             `json:"uncensor"`
	BeforeMeta  *model.MovieMeta `json:"before_meta"`
	AfterMeta   *model.MovieMeta `json:"after_meta"`
	Error       string           `json:"error"`
	Steps       []DebugStep      `json:"steps"`
}

type DebugHandlerOption struct {
	Disable bool
	Args    interface{}
}

type Debugger struct {
	deps      appdeps.Runtime
	cleaner   movieidcleaner.Cleaner
	instances []DebugHandlerInstance
	configs   map[string]DebugHandlerOption
}

func NewDebugger(deps appdeps.Runtime, cleaner movieidcleaner.Cleaner, handlers []string, configs map[string]DebugHandlerOption) *Debugger {
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

func (d *Debugger) Debug(ctx context.Context, req DebugRequest) (*DebugResult, error) {
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "single"
	}
	handlerID := strings.TrimSpace(req.HandlerID)
	if mode == "single" && handlerID == "" {
		return nil, fmt.Errorf("handler_id is required")
	}
	metaInput := req.Meta
	if metaInput == nil {
		metaInput = &model.MovieMeta{}
	}
	beforeMeta, err := cloneMovieMeta(metaInput)
	if err != nil {
		return nil, err
	}
	afterMeta, err := cloneMovieMeta(metaInput)
	if err != nil {
		return nil, err
	}
	num, err := d.parseNumber(afterMeta.Number)
	if err != nil {
		return nil, err
	}
	debugResult := &DebugResult{
		Mode:       mode,
		NumberID:   num.GetNumberID(),
		Category:   num.GetExternalFieldCategory(),
		Uncensor:   num.GetExternalFieldUncensor(),
		BeforeMeta: beforeMeta,
		AfterMeta:  afterMeta,
	}
	fc := &model.FileContext{
		Meta:   afterMeta,
		Number: num,
	}
	switch mode {
	case "single":
		instance, handlerCfg, lookupErr := d.lookupHandler(handlerID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		debugResult.HandlerID = instance.ID
		debugResult.HandlerName = instance.Name
		step, runErr := d.runOne(ctx, fc, *instance, handlerCfg)
		if runErr != nil {
			return nil, runErr
		}
		debugResult.Steps = []DebugStep{*step}
		debugResult.Error = step.Error
	case "chain":
		chain := d.resolveChain(req.HandlerIDs)
		failCount := 0
		for _, instance := range chain {
			step, runErr := d.runOne(ctx, fc, instance, d.configs[instance.ID])
			if runErr != nil {
				return nil, runErr
			}
			debugResult.Steps = append(debugResult.Steps, *step)
			if step.Error != "" {
				failCount++
			}
		}
		if failCount > 0 {
			debugResult.Error = fmt.Sprintf("%d handlers failed", failCount)
		}
	default:
		return nil, fmt.Errorf("invalid mode: %s", mode)
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

func (d *Debugger) runOne(ctx context.Context, fc *model.FileContext, instance DebugHandlerInstance, handlerCfg DebugHandlerOption) (*DebugStep, error) {
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
	return nil, DebugHandlerOption{}, fmt.Errorf("handler instance not found: %s", handlerID)
}

func (d *Debugger) parseNumber(rawInput string) (*number.Number, error) {
	input := strings.TrimSpace(rawInput)
	if input == "" {
		return nil, fmt.Errorf("meta.number is required")
	}
	if d.cleaner != nil {
		res, err := d.cleaner.Clean(input)
		if err == nil && res != nil && strings.TrimSpace(res.NumberID) != "" {
			num, parseErr := number.Parse(res.NumberID)
			if parseErr == nil {
				if res.Category != "" {
					num.SetExternalFieldCategory(res.Category)
				}
				if res.Uncensor {
					num.SetExternalFieldUncensor(true)
				}
				return num, nil
			}
		}
		var cleanErr *movieidcleaner.CleanError
		if err != nil && !errors.As(err, &cleanErr) {
			return nil, err
		}
	}
	return number.Parse(input)
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
