package bootstrap

import (
	"context"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

// logOptionalSetupFailure 打印"可选组件初始化失败"的错误日志。
//
// 适用场景: 翻译器、人脸识别等"没配好也能启动"的可选组件。
// 优先使用 sc.Infra.Logger (启动早期可能还是 nil), fallback 到 ctx logger,
// 保证在任何阶段都能留下可排障的记录。
func logOptionalSetupFailure(ctx context.Context, sc *StartContext, message string, err error) {
	if sc.Infra.Logger != nil {
		sc.Infra.Logger.Error(message, zap.Error(err))
		return
	}
	logutil.GetLogger(ctx).Error(message, zap.Error(err))
}
