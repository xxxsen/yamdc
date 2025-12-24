package dynscript

import (
	"context"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/picker"
	"go.uber.org/zap"
)

type NumberCategoryFunc = func(ctx context.Context, number string) (string, bool, error)

type INumberCategorier interface {
	Category(ctx context.Context, number string) (string, bool, error)
}

type numberCategorierImpl struct {
	pk picker.IPicker[NumberCategoryFunc]
}

func (n *numberCategorierImpl) Category(ctx context.Context, number string) (string, bool, error) {
	for _, name := range n.pk.List() {
		f, err := n.pk.Get(name)
		if err != nil {
			logutil.GetLogger(ctx).Error("get number category plugin failed", zap.String("name", name), zap.Error(err))
			continue
		}
		category, matched, err := f(ctx, number)
		if err != nil {
			logutil.GetLogger(ctx).Error("call number category plugin failed", zap.String("name", name), zap.Error(err))
			continue
		}
		if matched {
			return category, true, nil
		}
	}
	return "", false, nil
}

func NewNumberCategorier(rule string) (INumberCategorier, error) {
	rule = rewriteTabToSpace(rule)
	pk, err := picker.Parse[NumberCategoryFunc]([]byte(rule), picker.TomlDecoder, picker.WithSafeFuncWrap(true))
	if err != nil {
		return nil, err
	}
	return &numberCategorierImpl{
		pk: pk,
	}, nil
}
