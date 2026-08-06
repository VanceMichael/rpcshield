//go:build !linux

package wpamode

import (
	"runtime"

	"go.uber.org/zap"
)

func setMemoryLimit(fractionOfDRAM float64, logger *zap.SugaredLogger) error {
	logger.Infow("unsupported OS to set memory limit, expected linux", "os", runtime.GOOS)
	return nil
}
