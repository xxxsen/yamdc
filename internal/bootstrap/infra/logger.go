package infra

import (
	"github.com/xxxsen/common/logger"
	"go.uber.org/zap"
)

func InitLogger(file, level string, fileCount, fileSize, keepDays int, console bool) *zap.Logger {
	return logger.Init(file, level, fileCount, fileSize, keepDays, console)
}
