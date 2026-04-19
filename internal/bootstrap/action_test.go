package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/config"
)

func TestSortAutoReorders(t *testing.T) {
	actions := []InitAction{
		{Name: "c", Requires: []string{"x", "y"}},
		{Name: "a", Provides: []string{"x"}},
		{Name: "b", Requires: []string{"x"}, Provides: []string{"y"}},
	}
	sorted, err := Sort(actions)
	require.NoError(t, err)
	names := make([]string, len(sorted))
	for i, a := range sorted {
		names[i] = a.Name
	}
	require.Equal(t, []string{"a", "b", "c"}, names)
}

func TestSortPreservesOriginalOrderForUnconstrainedActions(t *testing.T) {
	actions := []InitAction{
		{Name: "d"},
		{Name: "a"},
		{Name: "c"},
		{Name: "b"},
	}
	sorted, err := Sort(actions)
	require.NoError(t, err)
	names := make([]string, len(sorted))
	for i, a := range sorted {
		names[i] = a.Name
	}
	require.Equal(t, []string{"d", "a", "c", "b"}, names)
}

func TestSortMissingDependency(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Provides: []string{"x"}},
		{Name: "b", Requires: []string{"missing"}},
	}
	_, err := Sort(actions)
	require.Error(t, err)
	assert.ErrorIs(t, err, errMissingDependency)
	assert.Contains(t, err.Error(), `action "b" requires "missing"`)
}

func TestSortDuplicateProvides(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Provides: []string{"x"}},
		{Name: "b", Provides: []string{"x"}},
	}
	_, err := Sort(actions)
	require.Error(t, err)
	assert.ErrorIs(t, err, errDuplicateProvide)
	assert.Contains(t, err.Error(), `action "b" provides "x"`)
	assert.Contains(t, err.Error(), `already provided by "a"`)
}

func TestSortSelfDependency(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Requires: []string{"x"}, Provides: []string{"x"}},
	}
	_, err := Sort(actions)
	require.Error(t, err)
	assert.ErrorIs(t, err, errCyclicDependency)
	assert.Contains(t, err.Error(), `action "a" requires "x"`)
}

func TestSortCyclicDependency(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Requires: []string{"y"}, Provides: []string{"x"}},
		{Name: "b", Requires: []string{"x"}, Provides: []string{"y"}},
	}
	_, err := Sort(actions)
	require.Error(t, err)
	assert.ErrorIs(t, err, errCyclicDependency)
}

func TestSortEmptyActions(t *testing.T) {
	sorted, err := Sort(nil)
	require.NoError(t, err)
	require.Nil(t, sorted)

	sorted, err = Sort([]InitAction{})
	require.NoError(t, err)
	require.Empty(t, sorted)
}

func TestValidateHappyPath(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Provides: []string{"x"}},
		{Name: "b", Requires: []string{"x"}, Provides: []string{"y"}},
		{Name: "c", Requires: []string{"x", "y"}},
	}
	require.NoError(t, Validate(actions))
}

func TestValidateOutOfOrderStillPasses(t *testing.T) {
	actions := []InitAction{
		{Name: "c", Requires: []string{"x", "y"}},
		{Name: "a", Provides: []string{"x"}},
		{Name: "b", Requires: []string{"x"}, Provides: []string{"y"}},
	}
	require.NoError(t, Validate(actions))
}

func TestValidateMissingDependency(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Provides: []string{"x"}},
		{Name: "b", Requires: []string{"missing"}},
	}
	err := Validate(actions)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `action "b" requires "missing"`)
}

func TestValidateDuplicateProvides(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Provides: []string{"x"}},
		{Name: "b", Provides: []string{"x"}},
	}
	err := Validate(actions)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `action "b" provides "x"`)
	assert.Contains(t, err.Error(), `already provided by "a"`)
}

func TestValidateEmptyActions(t *testing.T) {
	require.NoError(t, Validate(nil))
	require.NoError(t, Validate([]InitAction{}))
}

func TestValidateNoRequiresNoProvides(t *testing.T) {
	actions := []InitAction{
		{Name: "a"},
		{Name: "b"},
	}
	require.NoError(t, Validate(actions))
}

func TestExecuteAutoSorts(t *testing.T) {
	var order []string
	actions := []InitAction{
		{
			Name:     "second",
			Requires: []string{"a"},
			Run: func(_ context.Context, _ *StartContext) error {
				order = append(order, "second")
				return nil
			},
		},
		{
			Name:     "first",
			Provides: []string{"a"},
			Run: func(_ context.Context, _ *StartContext) error {
				order = append(order, "first")
				return nil
			},
		},
	}
	sc := NewStartContext(&config.Config{})
	require.NoError(t, Execute(context.Background(), sc, actions))
	require.Equal(t, []string{"first", "second"}, order)
}

func TestExecuteValidationFailure(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Requires: []string{"missing"}},
	}
	sc := NewStartContext(&config.Config{})
	err := Execute(context.Background(), sc, actions)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action validation failed")
}

func TestExecuteStopsOnFirstError(t *testing.T) {
	errBoom := errors.New("boom")
	ran := false
	actions := []InitAction{
		{
			Name:     "fail",
			Provides: []string{"a"},
			Run: func(_ context.Context, _ *StartContext) error {
				return errBoom
			},
		},
		{
			Name:     "should_not_run",
			Requires: []string{"a"},
			Run: func(_ context.Context, _ *StartContext) error {
				ran = true
				return nil
			},
		},
	}
	sc := NewStartContext(&config.Config{})
	err := Execute(context.Background(), sc, actions)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fail failed")
	assert.ErrorIs(t, err, errBoom)
	assert.False(t, ran)
}

func TestNewServerActionsPassesValidation(t *testing.T) {
	actions := NewServerActions()
	require.NoError(t, Validate(actions))
}

func TestProvidedResources(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Provides: []string{"x", "y"}},
		{Name: "b", Provides: []string{"z"}},
	}
	got := ProvidedResources(actions)
	require.Equal(t, []string{"x", "y", "z"}, got)
}

func TestRequiredResources(t *testing.T) {
	actions := []InitAction{
		{Name: "a", Requires: []string{"p"}},
		{Name: "b", Requires: []string{"p", "q"}},
	}
	got := RequiredResources(actions)
	require.Equal(t, []string{"p", "q"}, got)
}
