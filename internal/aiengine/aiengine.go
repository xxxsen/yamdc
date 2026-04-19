package aiengine

import "context"

type IAIEngine interface {
	Name() string
	Complete(ctx context.Context, prompt string, args map[string]any) (string, error)
}
