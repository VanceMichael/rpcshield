package http

import (
	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/grail"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
)

// UnknownEndpoint is the default value used when an endpoint cannot be determined.
const UnknownEndpoint = "<unknown>"

// MuxProvider defines the interface for HTTP mux providers that can identify
// mux creation and handler registration patterns in SSA instructions.
type MuxProvider interface {
	// MatchMuxRouterType checks if the given SSA value has the type of a mux router.
	MatchMuxRouterType(val ssa.Value) bool

	// MatchMuxCreation checks if the given SSA instruction represents the creation of a mux router.
	MatchMuxCreation(instr ssa.Instruction) bool

	// MatchHandlerRegistration checks if the given SSA instruction represents handler registration
	// and returns the corresponding InboundPoint if a match is found.
	MatchHandlerRegistration(instr ssa.Instruction) (ib []fty.InboundPoint, ok bool)
}

type baseProvider struct {
	logger   *zap.SugaredLogger
	service  grail.Service
	codebase *fty.Codebase
}
