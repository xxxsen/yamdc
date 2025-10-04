package dynscript

import (
	"context"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/picker"
	"go.uber.org/zap"
)

type NumberRewriteFunc = func(ctx context.Context, number string) (string, error)

type INumberRewriter interface {
	Rewrite(ctx context.Context, number string) (string, error)
}

type numberRewriterImpl struct {
	pk picker.IPicker[NumberRewriteFunc]
}

func (n *numberRewriterImpl) Rewrite(ctx context.Context, number string) (string, error) {
	for _, name := range n.pk.List() {
		f, err := n.pk.Get(name)
		if err != nil {
			logutil.GetLogger(ctx).Error("get number rewrite plugin failed", zap.String("name", name), zap.Error(err))
			continue
		}
		rewrited, err := f(ctx, number)
		if err != nil {
			logutil.GetLogger(ctx).Error("call number rewrite plugin failed", zap.String("name", name), zap.Error(err))
			continue
		}
		if len(rewrited) == 0 {
			continue
		}
		number = rewrited
	}
	return number, nil
}

func NewNumberRewriter(rule string) (INumberRewriter, error) {
	rule = rewriteTabToSpace(rule)
	pk, err := picker.Parse[NumberRewriteFunc]([]byte(rule), picker.TomlDecoder, picker.WithSafeFuncWrap(true))
	if err != nil {
		return nil, err
	}
	return &numberRewriterImpl{pk: pk}, nil
}
