package rpcshield

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// InboundTargetResolver resolves a service name to the Bazel targets that comprise it, so Driver
// knows which targets to lift/load and analyze for a given service name passed to Driver.Run.
//
// Callers who already know their Bazel targets (or have no service-name-to-target convention at
// all) should skip this interface entirely by supplying DriverOptions.Targets, or by calling
// Driver.RunOnGoPath/Driver.RunOnPackages instead of Driver.Run.
type InboundTargetResolver interface {
	FindTargets(serviceName string) ([]string, error)
}

// pinocchioTargetResolver resolves targets by reading a Pinocchio-style deployment YAML convention
// (<workspaceRoot>/udeploy/pinocchio/<serviceName>/production.yaml, go_standard.bazel_targets).
// This is the default InboundTargetResolver used by both the OSS CLI and Uber's monorepo wiring
// unless a caller supplies their own InboundTargetResolver.
type pinocchioTargetResolver struct {
	workspaceRoot string
	logger        *zap.SugaredLogger
}

// NewPinocchioTargetResolver returns the default InboundTargetResolver, which reads Bazel targets
// from a Pinocchio-style deployment YAML convention.
func NewPinocchioTargetResolver(workspaceRoot string, logger *zap.SugaredLogger) InboundTargetResolver {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &pinocchioTargetResolver{workspaceRoot: workspaceRoot, logger: logger}
}

// FindTargets implements InboundTargetResolver.
func (r *pinocchioTargetResolver) FindTargets(serviceName string) ([]string, error) {
	r.logger.Infow("Reading pinnochio YAML file for service", "service", serviceName)
	file := filepath.Join(r.workspaceRoot, "udeploy", "pinocchio", serviceName, "production.yaml")

	// If Pinocchio deployment YAML file exists we can read from it.
	reader, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", file, err)
	}
	defer reader.Close()

	var config struct {
		GoStandard struct {
			BazelTargets []string `yaml:"bazel_targets"`
		} `yaml:"go_standard"`
	}
	if err := yaml.NewDecoder(reader).Decode(&config); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", file, err)
	}
	return config.GoStandard.BazelTargets, nil
}
