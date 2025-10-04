package dynscript

import (
	"context"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/picker"
	"go.uber.org/zap"
)

type NumberUncensorCheckFunc = func(ctx context.Context, number string) (bool, error)

type INumberUncensorChecker interface {
	IsMatch(ctx context.Context, number string) (bool, error)
}

type uncensorCheckerImpl struct {
	pk picker.IPicker[NumberUncensorCheckFunc]
}

func (u *uncensorCheckerImpl) IsMatch(ctx context.Context, number string) (bool, error) {
	for _, name := range u.pk.List() {
		f, err := u.pk.Get(name)
		if err != nil {
			logutil.GetLogger(ctx).Error("get uncensor check plugin failed", zap.String("name", name), zap.Error(err))
			continue
		}
		matched, err := f(ctx, number)
		if err != nil {
			logutil.GetLogger(ctx).Error("call uncensor check plugin failed", zap.String("name", name), zap.Error(err))
			continue
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func NewNumberUncensorChecker(rule string) (INumberUncensorChecker, error) {
	rule = rewriteTabToSpace(rule)
	pk, err := picker.Parse[NumberUncensorCheckFunc]([]byte(rule), picker.TomlDecoder, picker.WithSafeFuncWrap(true))
	if err != nil {
		return nil, err
	}
	return &uncensorCheckerImpl{
		pk: pk,
	}, nil
}
