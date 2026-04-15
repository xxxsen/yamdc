package movieidcleaner

import (
	"fmt"
	"sync"
)

type ISwapableCleaner interface {
	Swap(Cleaner)
}

type RuntimeCleaner struct {
	mu    sync.RWMutex
	inner Cleaner
}

func NewRuntimeCleaner(inner Cleaner) *RuntimeCleaner {
	if inner == nil {
		inner = NewPassthroughCleaner()
	}
	return &RuntimeCleaner{inner: inner}
}

func (r *RuntimeCleaner) Clean(input string) (*Result, error) {
	r.mu.RLock()
	inner := r.inner
	r.mu.RUnlock()
	res, err := inner.Clean(input)
	if err != nil {
		return nil, fmt.Errorf("runtime clean failed: %w", err)
	}
	return res, nil
}

func (r *RuntimeCleaner) Explain(input string) (*ExplainResult, error) {
	r.mu.RLock()
	inner := r.inner
	r.mu.RUnlock()
	res, err := inner.Explain(input)
	if err != nil {
		return nil, fmt.Errorf("runtime explain failed: %w", err)
	}
	return res, nil
}

func (r *RuntimeCleaner) Swap(inner Cleaner) {
	if inner == nil {
		return
	}
	r.mu.Lock()
	r.inner = inner
	r.mu.Unlock()
}
