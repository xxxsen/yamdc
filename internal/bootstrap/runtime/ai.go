package runtime

import (
	"context"
	"fmt"

	"github.com/xxxsen/common/logutil"

	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/client"
)

func BuildAIEngine(
	ctx context.Context,
	cli client.IHTTPClient,
	name string,
	args any,
) (aiengine.IAIEngine, error) {
	if name == "" {
		logutil.GetLogger(ctx).Info("ai engine is disabled, skip init")
		return nil, ErrAIEngineNotConfigured
	}
	engine, err := aiengine.Create(name, args, aiengine.WithHTTPClient(cli))
	if err != nil {
		return nil, fmt.Errorf("create ai engine failed, name:%s, err:%w", name, err)
	}
	return engine, nil
}
