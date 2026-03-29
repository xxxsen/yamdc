package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/config"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
)

type DebugHandlerInstance struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DebugRequest struct {
	HandlerID string           `json:"handler_id"`
	Meta      *model.MovieMeta `json:"meta"`
}

type DebugResult struct {
	HandlerID   string           `json:"handler_id"`
	HandlerName string           `json:"handler_name"`
	NumberID    string           `json:"number_id"`
	Category    string           `json:"category"`
	Uncensor    bool             `json:"uncensor"`
	BeforeMeta  *model.MovieMeta `json:"before_meta"`
	AfterMeta   *model.MovieMeta `json:"after_meta"`
	Error       string           `json:"error"`
}

type Debugger struct {
	deps      appdeps.Runtime
	cleaner   numbercleaner.Cleaner
	instances []DebugHandlerInstance
	configs   map[string]config.HandlerConfig
}

func NewDebugger(deps appdeps.Runtime, cleaner numbercleaner.Cleaner, handlers []string, configs map[string]config.HandlerConfig) *Debugger {
	items := make([]DebugHandlerInstance, 0, len(handlers))
	cfgMap := make(map[string]config.HandlerConfig, len(handlers))
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
	handlerID := strings.TrimSpace(req.HandlerID)
	if handlerID == "" {
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
	instance, handlerCfg, err := d.lookupHandler(handlerID)
	if err != nil {
		return nil, err
	}
	num, err := d.parseNumber(afterMeta.Number)
	if err != nil {
		return nil, err
	}
	fc := &model.FileContext{
		Meta:   afterMeta,
		Number: num,
	}
	h, err := CreateHandler(instance.Name, handlerCfg.Args, d.deps)
	if err != nil {
		return nil, err
	}
	debugResult := &DebugResult{
		HandlerID:   instance.ID,
		HandlerName: instance.Name,
		NumberID:    num.GetNumberID(),
		Category:    num.GetExternalFieldCategory(),
		Uncensor:    num.GetExternalFieldUncensor(),
		BeforeMeta:  beforeMeta,
		AfterMeta:   afterMeta,
	}
	if err := h.Handle(ctx, fc); err != nil {
		debugResult.Error = err.Error()
	}
	return debugResult, nil
}

func (d *Debugger) lookupHandler(handlerID string) (*DebugHandlerInstance, config.HandlerConfig, error) {
	for _, item := range d.instances {
		if item.ID == handlerID {
			return &item, d.configs[handlerID], nil
		}
	}
	return nil, config.HandlerConfig{}, fmt.Errorf("handler instance not found: %s", handlerID)
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
		var cleanErr *numbercleaner.CleanError
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
