package bazel

import (
	"github.com/uber-research/rpcshield/go/common/utils"
)

// GoPathWrapper is the interface for running Bazel commands.
type GoPathWrapper interface {
	LiftGoPath(runner utils.Runner, target string) (string, []string, error)
}

// FSRenamer interface to abstract the Rename method.
type FSRenamer interface {
	Rename(gopath, path string) error
}
