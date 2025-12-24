package aiengine

import "context"

var (
	defaultAIEngine IAIEngine
)

type IAIEngine interface {
	Name() string
	Complete(ctx context.Context, prompt string, args map[string]interface{}) (string, error)
}

func SetAIEngine(engine IAIEngine) {
	defaultAIEngine = engine
}

func Complete(ctx context.Context, prompt string, args map[string]interface{}) (string, error) {
	return defaultAIEngine.Complete(ctx, prompt, args)
}

func IsAIEngineEnabled() bool {
	return defaultAIEngine != nil
}
