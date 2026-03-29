package numbercleaner

import "sync"

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
	return inner.Clean(input)
}

func (r *RuntimeCleaner) Swap(inner Cleaner) {
	if inner == nil {
		return
	}
	r.mu.Lock()
	r.inner = inner
	r.mu.Unlock()
}
