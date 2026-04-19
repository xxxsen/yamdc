package runtime

import (
	"context"
	"strings"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/translator"
	"github.com/xxxsen/yamdc/internal/translator/ai"
	"github.com/xxxsen/yamdc/internal/translator/google"
)

type TranslatorConfig struct {
	Engine   string
	Fallback []string
	Proxy    string
	Google   GoogleTranslatorConfig
	AI       AITranslatorConfig
}

type GoogleTranslatorConfig struct {
	Enable   bool
	UseProxy bool
}

type AITranslatorConfig struct {
	Enable bool
	Prompt string
}

func BuildTranslator(
	ctx context.Context,
	cfg TranslatorConfig,
	engine aiengine.IAIEngine,
) (translator.ITranslator, error) {
	allEngines := make(map[string]translator.ITranslator, 2)
	if cfg.Google.Enable {
		opts := []google.Option{}
		if cfg.Google.UseProxy && cfg.Proxy != "" {
			opts = append(opts, google.WithProxyURL(cfg.Proxy))
		}
		allEngines[translator.TrNameGoogle] = google.New(opts...)
	}
	if cfg.AI.Enable {
		allEngines[translator.TrNameAI] = ai.New(engine, ai.WithPrompt(cfg.AI.Prompt))
	}
	engineNames := make([]string, 0, 1+len(cfg.Fallback))
	engineNames = append(engineNames, cfg.Engine)
	engineNames = append(engineNames, cfg.Fallback...)
	useEngines := make([]translator.ITranslator, 0, len(engineNames))
	seen := make(map[string]struct{}, len(engineNames))
	for _, name := range engineNames {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		e, ok := allEngines[name]
		if !ok {
			logutil.GetLogger(ctx).Error("spec engine not found, skip", zap.String("name", name))
			continue
		}
		useEngines = append(useEngines, e)
	}
	if len(useEngines) == 0 {
		return nil, ErrNoEngineUsed
	}
	return translator.NewGroup(useEngines...), nil
}
