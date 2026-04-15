package processor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
)

type mockHandler struct {
	err error
}

func (h *mockHandler) Handle(_ context.Context, _ *model.FileContext) error {
	return h.err
}

func TestDefaultProcessorName(t *testing.T) {
	assert.Equal(t, "default", DefaultProcessor.Name())
}

func TestDefaultProcessorProcess(t *testing.T) {
	err := DefaultProcessor.Process(context.Background(), &model.FileContext{})
	assert.NoError(t, err)
}

func TestNewProcessor(t *testing.T) {
	tests := []struct {
		name       string
		procName   string
		handlerErr error
		wantErr    bool
	}{
		{
			name:     "success",
			procName: "test_proc",
		},
		{
			name:       "handler error",
			procName:   "err_proc",
			handlerErr: errors.New("handler fail"),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProcessor(tt.procName, &mockHandler{err: tt.handlerErr})
			assert.Equal(t, tt.procName, p.Name())
			err := p.Process(context.Background(), &model.FileContext{
				Meta: &model.MovieMeta{},
			})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGroup(t *testing.T) {
	tests := []struct {
		name       string
		processors []IProcessor
		wantErr    bool
	}{
		{
			name:       "empty group",
			processors: nil,
		},
		{
			name: "all success",
			processors: []IProcessor{
				NewProcessor("p1", &mockHandler{}),
				NewProcessor("p2", &mockHandler{}),
			},
		},
		{
			name: "one failure continues others",
			processors: []IProcessor{
				NewProcessor("p1", &mockHandler{err: errors.New("fail")}),
				NewProcessor("p2", &mockHandler{}),
			},
			wantErr: true,
		},
		{
			name: "multiple failures returns last error",
			processors: []IProcessor{
				NewProcessor("p1", &mockHandler{err: errors.New("fail1")}),
				NewProcessor("p2", &mockHandler{err: errors.New("fail2")}),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGroup(tt.processors)
			require.Equal(t, "group", g.Name())
			err := g.Process(context.Background(), &model.FileContext{
				Meta: &model.MovieMeta{},
			})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGroupContinuesAfterError(t *testing.T) {
	var callOrder []string
	p1 := &trackingProcessor{name: "p1", err: errors.New("p1 fail"), calls: &callOrder}
	p2 := &trackingProcessor{name: "p2", calls: &callOrder}
	p3 := &trackingProcessor{name: "p3", calls: &callOrder}

	g := NewGroup([]IProcessor{p1, p2, p3})
	err := g.Process(context.Background(), &model.FileContext{Meta: &model.MovieMeta{}})
	assert.Error(t, err)
	assert.Equal(t, []string{"p1", "p2", "p3"}, callOrder)
}

type trackingProcessor struct {
	name  string
	err   error
	calls *[]string
}

func (p *trackingProcessor) Name() string { return p.name }
func (p *trackingProcessor) Process(_ context.Context, _ *model.FileContext) error {
	*p.calls = append(*p.calls, p.name)
	return p.err
}
