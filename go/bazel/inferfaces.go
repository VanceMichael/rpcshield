package bazel

import (
	"context"

	"github.com/uber-research/rpcshield/go/common/utils"
)

// GoPathWrapper is the interface for running Bazel commands.
type GoPathWrapper interface {
	LiftGoPath(runner utils.Runner, target string) (string, []string, error)
}

// ContextGoPathWrapper is the optional, context-aware extension of
// GoPathWrapper. Implementations that support cancellation (the real
// *gopathWrapper does) kill long-running sub-commands -- notably the
// `bazel build` at the heart of GOPATH lifting -- when ctx is canceled and
// return the context error. Callers (the driver) use a type assertion to
// opt in; implementations that only satisfy GoPathWrapper keep working
// unchanged (the driver checks ctx.Err() around them instead).
type ContextGoPathWrapper interface {
	GoPathWrapper
	LiftGoPathContext(ctx context.Context, runner utils.Runner, target string) (string, []string, error)
}

// FSRenamer interface to abstract the Rename method.
type FSRenamer interface {
	Rename(gopath, path string) error
}
