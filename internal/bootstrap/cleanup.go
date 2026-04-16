package bootstrap

import (
	"context"
	"errors"
	"sync"
)

type cleanupEntry struct {
	name string
	fn   func(context.Context) error
}

type CleanupManager struct {
	mu       sync.Mutex
	cleanups []cleanupEntry
}

func NewCleanupManager() *CleanupManager {
	return &CleanupManager{}
}

func (m *CleanupManager) Add(name string, fn func(context.Context) error) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanups = append(m.cleanups, cleanupEntry{name: name, fn: fn})
}

// Run executes all registered cleanup functions in reverse order.
// All cleanups run regardless of individual failures; errors are aggregated.
func (m *CleanupManager) Run(ctx context.Context) error {
	m.mu.Lock()
	entries := make([]cleanupEntry, len(m.cleanups))
	copy(entries, m.cleanups)
	m.mu.Unlock()

	var errs []error
	for i := len(entries) - 1; i >= 0; i-- {
		if err := entries[i].fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
