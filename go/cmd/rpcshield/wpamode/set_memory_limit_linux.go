//go:build linux

package wpamode

import (
	"fmt"
	"syscall"

	"go.uber.org/zap"
)

func setMemoryLimit(fractionOfDRAM float64, logger *zap.SugaredLogger) error {
	if fractionOfDRAM <= 0 || fractionOfDRAM > 1 {
		return fmt.Errorf("invalid memory limit fraction: %f", fractionOfDRAM)
	}
	// Read the DRAM size from syscall.Sysinfo_t.
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		logger.Fatalf("failed to read system information: %v", err)
	}
	logger.Infow("system information", "totalram", info.Totalram, "freeram", info.Freeram, "bufferram", info.Bufferram)
	// Set the memory limit to specified fraction of the DRAM size.
	memoryLimit := uint64(fractionOfDRAM * float64(info.Totalram))
	if err := syscall.Setrlimit(syscall.RLIMIT_AS, &syscall.Rlimit{
		Cur: memoryLimit,
		Max: memoryLimit,
	}); err != nil {
		logger.Fatalf("failed to set memory limit: %v", err)
	}
	logger.Infow("memory limit set", "memory_limit", memoryLimit)

	return nil
}
