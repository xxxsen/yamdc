package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
)

func TestNewDebugger(t *testing.T) {
	tests := []struct {
		name      string
		handlers  []string
		configs   map[string]DebugHandlerOption
		wantCount int
	}{
		{
			name:      "empty handlers",
			handlers:  nil,
			configs:   nil,
			wantCount: 0,
		},
		{
			name:      "filters out duration_fixer",
			handlers:  []string{HNumberTitle, HDurationFixer, HTagPadder},
			configs:   nil,
			wantCount: 2,
		},
		{
			name:     "filters out disabled handlers",
			handlers: []string{HNumberTitle, HTagPadder},
			configs: map[string]DebugHandlerOption{
				HTagPadder: {Disable: true},
			},
			wantCount: 1,
		},
		{
			name:     "includes handler with config",
			handlers: []string{HNumberTitle},
			configs: map[string]DebugHandlerOption{
				HNumberTitle: {Args: nil},
			},
			wantCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDebugger(appdeps.Runtime{}, nil, tt.handlers, tt.configs)
			assert.Len(t, d.Handlers(), tt.wantCount)
		})
	}
}

func TestDebuggerHandlersReturnsCopy(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle}, nil)
	h1 := d.Handlers()
	h2 := d.Handlers()
	assert.Equal(t, h1, h2)
	h1[0].ID = "modified"
	assert.NotEqual(t, h1[0].ID, d.Handlers()[0].ID)
}

func TestDebugSingleMode(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle, HTagPadder}, nil)

	tests := []struct {
		name    string
		req     DebugRequest
		wantErr bool
	}{
		{
			name: "success",
			req: DebugRequest{
				Mode:      "single",
				HandlerID: HNumberTitle,
				Meta:      &model.MovieMeta{Number: "ABC-123", Title: "Test Movie"},
			},
		},
		{
			name: "missing handler_id in single mode",
			req: DebugRequest{
				Mode: "single",
				Meta: &model.MovieMeta{Number: "ABC-123"},
			},
			wantErr: true,
		},
		{
			name: "handler not found",
			req: DebugRequest{
				Mode:      "single",
				HandlerID: "nonexistent",
				Meta:      &model.MovieMeta{Number: "ABC-123", Title: "Test"},
			},
			wantErr: true,
		},
		{
			name: "empty mode defaults to single, missing handler_id",
			req: DebugRequest{
				Meta: &model.MovieMeta{Number: "ABC-123"},
			},
			wantErr: true,
		},
		{
			name: "missing meta number",
			req: DebugRequest{
				Mode:      "single",
				HandlerID: HNumberTitle,
				Meta:      &model.MovieMeta{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := d.Debug(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, "single", result.Mode)
			}
		})
	}
}

func TestDebugChainMode(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle, HTagPadder, HActorSpliter}, nil)

	tests := []struct {
		name      string
		req       DebugRequest
		wantSteps int
		wantErr   bool
	}{
		{
			name: "chain all",
			req: DebugRequest{
				Mode: "chain",
				Meta: &model.MovieMeta{Number: "ABC-123", Title: "Test"},
			},
			wantSteps: 3,
		},
		{
			name: "chain with specific handlers",
			req: DebugRequest{
				Mode:       "chain",
				HandlerIDs: []string{HNumberTitle, HActorSpliter},
				Meta:       &model.MovieMeta{Number: "ABC-123", Title: "Test"},
			},
			wantSteps: 2,
		},
		{
			name: "chain with empty handler IDs uses all",
			req: DebugRequest{
				Mode:       "chain",
				HandlerIDs: []string{},
				Meta:       &model.MovieMeta{Number: "ABC-123", Title: "Test"},
			},
			wantSteps: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := d.Debug(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "chain", result.Mode)
				assert.Len(t, result.Steps, tt.wantSteps)
			}
		})
	}
}

func TestDebugInvalidMode(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle}, nil)
	result, err := d.Debug(context.Background(), DebugRequest{
		Mode:      "invalid_mode",
		HandlerID: HNumberTitle,
		Meta:      &model.MovieMeta{Number: "ABC-123", Title: "Test"},
	})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errInvalidMode)
}

func TestDebugNilMeta(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle}, nil)
	_, err := d.Debug(context.Background(), DebugRequest{
		Mode:      "single",
		HandlerID: HNumberTitle,
		Meta:      nil,
	})
	assert.Error(t, err)
}

func TestCloneMovieMeta(t *testing.T) {
	tests := []struct {
		name  string
		input *model.MovieMeta
	}{
		{
			name:  "nil meta",
			input: nil,
		},
		{
			name: "full meta",
			input: &model.MovieMeta{
				Title:  "Test",
				Number: "ABC-123",
				Actors: []string{"Actor1"},
				Genres: []string{"Genre1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := cloneMovieMeta(tt.input)
			require.NoError(t, err)
			assert.NotNil(t, out)
			if tt.input != nil {
				assert.Equal(t, tt.input.Title, out.Title)
				assert.Equal(t, tt.input.Number, out.Number)
			}
		})
	}
}

func TestCleanNumberError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantNil bool
	}{
		{
			name:    "CleanError returns nil",
			err:     &movieidcleaner.CleanError{Code: "test", Message: "test"},
			wantNil: true,
		},
		{
			name:    "other error returns wrapped",
			err:     errors.New("random error"),
			wantNil: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanNumberError(tt.err)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.Error(t, result)
			}
		})
	}
}

func TestBuildNumberFromCleanResult(t *testing.T) {
	tests := []struct {
		name    string
		result  *movieidcleaner.Result
		wantNil bool
	}{
		{
			name:    "nil result",
			result:  nil,
			wantNil: true,
		},
		{
			name:    "empty number ID",
			result:  &movieidcleaner.Result{NumberID: ""},
			wantNil: true,
		},
		{
			name:    "whitespace number ID",
			result:  &movieidcleaner.Result{NumberID: "  "},
			wantNil: true,
		},
		{
			name: "valid number",
			result: &movieidcleaner.Result{
				NumberID: "ABC-123",
				Category: "testcat",
				Uncensor: true,
			},
			wantNil: false,
		},
		{
			name:    "invalid number (has dot)",
			result:  &movieidcleaner.Result{NumberID: "invalid.mp4"},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, err := buildNumberFromCleanResult(tt.result)
			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, num)
			} else {
				assert.NotNil(t, num)
				if tt.result.Category != "" {
					assert.Equal(t, tt.result.Category, num.GetExternalFieldCategory())
				}
				if tt.result.Uncensor {
					assert.True(t, num.GetExternalFieldUncensor())
				}
			}
		})
	}
}

type testCleaner struct {
	result *movieidcleaner.Result
	err    error
}

func (c *testCleaner) Clean(_ string) (*movieidcleaner.Result, error) {
	return c.result, c.err
}

func (c *testCleaner) Explain(_ string) (*movieidcleaner.ExplainResult, error) {
	return nil, nil //nolint:nilnil
}

func TestParseNumberWithCleaner(t *testing.T) {
	tests := []struct {
		name    string
		cleaner movieidcleaner.Cleaner
		input   string
		wantErr bool
	}{
		{
			name:    "empty input",
			cleaner: nil,
			input:   "",
			wantErr: true,
		},
		{
			name:    "no cleaner, valid number",
			cleaner: nil,
			input:   "ABC-123",
			wantErr: false,
		},
		{
			name: "cleaner returns valid number",
			cleaner: &testCleaner{
				result: &movieidcleaner.Result{NumberID: "DEF-456"},
			},
			input:   "anything",
			wantErr: false,
		},
		{
			name: "cleaner returns CleanError, falls through to raw parse",
			cleaner: &testCleaner{
				err: &movieidcleaner.CleanError{Code: "test", Message: "test"},
			},
			input:   "XYZ-789",
			wantErr: false,
		},
		{
			name: "cleaner returns nil result, falls through",
			cleaner: &testCleaner{
				result: nil,
			},
			input:   "GHI-012",
			wantErr: false,
		},
		{
			name: "cleaner returns non-CleanError",
			cleaner: &testCleaner{
				err: errors.New("fatal"),
			},
			input:   "ABC-123",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Debugger{cleaner: tt.cleaner}
			num, err := d.parseNumber(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, num)
			}
		})
	}
}

func TestResolveChain(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle, HTagPadder, HActorSpliter}, nil)

	tests := []struct {
		name       string
		handlerIDs []string
		wantCount  int
	}{
		{
			name:       "nil uses all",
			handlerIDs: nil,
			wantCount:  3,
		},
		{
			name:       "empty uses all",
			handlerIDs: []string{},
			wantCount:  3,
		},
		{
			name:       "specific handlers",
			handlerIDs: []string{HNumberTitle},
			wantCount:  1,
		},
		{
			name:       "whitespace handler IDs ignored",
			handlerIDs: []string{"  ", HNumberTitle},
			wantCount:  1,
		},
		{
			name:       "nonexistent handler filtered",
			handlerIDs: []string{"nonexistent"},
			wantCount:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := d.resolveChain(tt.handlerIDs)
			assert.Len(t, chain, tt.wantCount)
		})
	}
}

func TestDebugChainWithFailures(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle, HTagPadder}, nil)
	result, err := d.Debug(context.Background(), DebugRequest{
		Mode: "chain",
		Meta: &model.MovieMeta{Number: "ABC-123", Title: "Test"},
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDebugSingleHandlerExecutionError(t *testing.T) {
	name := "test_runtime_error_handler"
	Register(name, func(_ interface{}, _ appdeps.Runtime) (IHandler, error) {
		return &failingHandler{}, nil
	})
	d := NewDebugger(appdeps.Runtime{}, nil, []string{name}, nil)
	result, err := d.Debug(context.Background(), DebugRequest{
		Mode:      "single",
		HandlerID: name,
		Meta:      &model.MovieMeta{Number: "ABC-123", Title: "Test"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
	assert.Len(t, result.Steps, 1)
	assert.NotEmpty(t, result.Steps[0].Error)
}

func TestDebugWithCleaner(t *testing.T) {
	cleaner := &testCleaner{
		result: &movieidcleaner.Result{
			NumberID: "DEF-456",
			Category: "testcat",
			Uncensor: true,
		},
	}
	d := NewDebugger(appdeps.Runtime{}, cleaner, []string{HNumberTitle}, nil)
	result, err := d.Debug(context.Background(), DebugRequest{
		Mode:      "single",
		HandlerID: HNumberTitle,
		Meta:      &model.MovieMeta{Number: "ABC-123", Title: "Test"},
	})
	require.NoError(t, err)
	assert.Equal(t, "DEF-456", result.NumberID)
	assert.Equal(t, "testcat", result.Category)
	assert.True(t, result.Uncensor)
}

func TestDebugEmptyModeWithHandlerID(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle}, nil)
	result, err := d.Debug(context.Background(), DebugRequest{
		Mode:      "",
		HandlerID: HNumberTitle,
		Meta:      &model.MovieMeta{Number: "ABC-123", Title: "Test"},
	})
	require.NoError(t, err)
	assert.Equal(t, "single", result.Mode)
	assert.Len(t, result.Steps, 1)
}

func TestDebugSingleHandlerWithHandlerID(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle}, nil)
	result, err := d.Debug(context.Background(), DebugRequest{
		Mode:      "single",
		HandlerID: HNumberTitle,
		Meta:      &model.MovieMeta{Number: "XYZ-999", Title: "Some Title"},
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.BeforeMeta)
	assert.NotNil(t, result.AfterMeta)
	assert.Equal(t, "single", result.Mode)
	assert.Equal(t, HNumberTitle, result.HandlerID)
}

func TestDebugChainWithHandlerErrors(t *testing.T) {
	Register("test_error_handler", func(_ interface{}, _ appdeps.Runtime) (IHandler, error) {
		return nil, errors.New("cannot create handler")
	})
	d := NewDebugger(appdeps.Runtime{}, nil, []string{"test_error_handler"}, nil)
	_, err := d.Debug(context.Background(), DebugRequest{
		Mode: "chain",
		Meta: &model.MovieMeta{Number: "ABC-123", Title: "Test"},
	})
	assert.Error(t, err)
}

func TestDebugSingleWithCreateHandlerError(t *testing.T) {
	Register("test_create_error", func(_ interface{}, _ appdeps.Runtime) (IHandler, error) {
		return nil, errors.New("creation error")
	})
	d := NewDebugger(appdeps.Runtime{}, nil, []string{"test_create_error"}, nil)
	_, err := d.Debug(context.Background(), DebugRequest{
		Mode:      "single",
		HandlerID: "test_create_error",
		Meta:      &model.MovieMeta{Number: "ABC-123", Title: "Test"},
	})
	assert.Error(t, err)
}

func TestPrepareDebugContextNilMeta(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle}, nil)
	_, _, err := d.prepareDebugContext(DebugRequest{
		Meta: nil,
	})
	assert.Error(t, err)
}

func TestDebugChainWithHandlerFailures(t *testing.T) {
	name := "test_failing_handler_chain"
	Register(name, func(_ interface{}, _ appdeps.Runtime) (IHandler, error) {
		return &failingHandler{}, nil
	})
	d := NewDebugger(appdeps.Runtime{}, nil, []string{name}, nil)
	result, err := d.Debug(context.Background(), DebugRequest{
		Mode: "chain",
		Meta: &model.MovieMeta{Number: "ABC-123", Title: "Test"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
	assert.Len(t, result.Steps, 1)
	assert.NotEmpty(t, result.Steps[0].Error)
}

type failingHandler struct{}

func (h *failingHandler) Handle(_ context.Context, _ *model.FileContext) error {
	return errors.New("handler execution failed")
}

func TestLookupHandler(t *testing.T) {
	d := NewDebugger(appdeps.Runtime{}, nil, []string{HNumberTitle}, nil)

	t.Run("found", func(t *testing.T) {
		inst, _, err := d.lookupHandler(HNumberTitle)
		require.NoError(t, err)
		assert.Equal(t, HNumberTitle, inst.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, _, err := d.lookupHandler("nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, errHandlerInstanceNotFound)
	})
}
