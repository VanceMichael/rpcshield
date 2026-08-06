package ssaanal

import "golang.org/x/tools/go/ssa"

// rootSourceKind categorizes root error sources in the analyzed function.
type rootSourceKind int

const (
	rootSrc       rootSourceKind = iota // Primary RPC error being analyzed
	rootOtherSrc                        // Other tracked error-producing calls
	rootSynthetic                       // Synthetic error creators (errors.New, fmt.Errorf)
)

// rootSourceRegistry holds all root error sources identified during initialization.
// Root sources have their valState set once and never modified during SCCP.
// Immutable after construction.
type rootSourceRegistry struct {
	byValue map[ssa.Value]rootSourceKind
	src     sourceID        // The primary source call being analyzed
	srcV    ssa.Value       // The error value from src (same underlying value, typed as ssa.Value)
	srcIn   ssa.Instruction // The instruction producing the src error
}

func newRootSourceRegistry(src sourceID, srcIns ssa.Instruction) *rootSourceRegistry {
	return &rootSourceRegistry{
		byValue: make(map[ssa.Value]rootSourceKind),
		src:     src,
		srcV:    ssa.Value(src),
		srcIn:   srcIns,
	}
}

func (r *rootSourceRegistry) add(v ssa.Value, kind rootSourceKind) {
	if _, exists := r.byValue[v]; !exists {
		r.byValue[v] = kind
	}
}

func (r *rootSourceRegistry) lookup(v ssa.Value) (rootSourceKind, bool) {
	kind, ok := r.byValue[v]
	return kind, ok
}

func (r *rootSourceRegistry) isRoot(v ssa.Value) bool {
	_, ok := r.byValue[v]
	return ok
}

// srcID returns the sourceID of the primary source being analyzed.
func (r *rootSourceRegistry) srcID() sourceID {
	return r.src
}

// srcVal returns the ssa.Value of the primary source error.
func (r *rootSourceRegistry) srcVal() ssa.Value {
	return r.srcV
}

// srcBlock returns the basic block containing the primary source instruction.
func (r *rootSourceRegistry) srcBlock() *ssa.BasicBlock {
	return r.srcIn.Block()
}
