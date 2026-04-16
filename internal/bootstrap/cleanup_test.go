package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupManagerRunsInReverseOrder(t *testing.T) {
	m := NewCleanupManager()
	var order []string
	m.Add("first", func(context.Context) error {
		order = append(order, "first")
		return nil
	})
	m.Add("second", func(context.Context) error {
		order = append(order, "second")
		return nil
	})
	m.Add("third", func(context.Context) error {
		order = append(order, "third")
		return nil
	})
	err := m.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"third", "second", "first"}, order)
}

func TestCleanupManagerAggregatesAllErrors(t *testing.T) {
	m := NewCleanupManager()
	err1 := errors.New("error one")
	err2 := errors.New("error two")
	m.Add("ok", func(context.Context) error { return nil })
	m.Add("fail1", func(context.Context) error { return err1 })
	m.Add("fail2", func(context.Context) error { return err2 })

	err := m.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, err1)
	assert.ErrorIs(t, err, err2)
}

func TestCleanupManagerContinuesAfterError(t *testing.T) {
	m := NewCleanupManager()
	lastRan := false
	m.Add("first_registered", func(context.Context) error {
		lastRan = true
		return nil
	})
	m.Add("will_fail", func(context.Context) error {
		return errors.New("kaboom")
	})

	err := m.Run(context.Background())
	require.Error(t, err)
	assert.True(t, lastRan, "cleanup registered first should still run after later cleanup fails")
}

func TestCleanupManagerNilFuncIgnored(t *testing.T) {
	m := NewCleanupManager()
	m.Add("nil", nil)
	require.NoError(t, m.Run(context.Background()))
}

func TestCleanupManagerEmpty(t *testing.T) {
	m := NewCleanupManager()
	require.NoError(t, m.Run(context.Background()))
}
