package ssaanal

// =============================================================================
// Abstract Lattices for SCCP Analysis
// =============================================================================
//
// This file defines the abstract lattices used in Sparse Conditional Constant
// Propagation (SCCP) analysis. A lattice is a partially ordered set with:
//   - A bottom element (⊥): represents "not yet computed"
//   - A top element (⊤): represents "unknown/could be anything"
//   - Concrete values: represent known facts
//   - A join operation: computes the least upper bound (merge values)
//
// We use three lattices:
//   1. Nilness lattice: tracks whether a value is nil, non-nil, or unknown
//   2. Boolean constant lattice: tracks whether a boolean is true, false, or unknown
//   3. Synthetic lattice: tracks whether a value is created by a synthetic error creator
//
// Example nilness lattice usage:
//
//	⊥.join(nil) = nil           // Initially bottom, becomes nil
//	nil.join(nil) = nil         // Stays nil
//	nil.join(non-nil) = ⊤       // Conflicting values → unknown
//	⊤.join(anything) = ⊤        // Once unknown, stays unknown
//
// =============================================================================

// =============================================================================
// Nilness Lattice - for interface/pointer/error values
// =============================================================================

// nilness represents the abstract nil-ness of an interface/pointer value.
// This is ONLY used for values that can be nil (errors, interfaces, pointers).
type nilness int

const (
	nlBottom nilness = iota // Not yet computed
	nlNil                   // Definitely nil
	nlNonNil                // Definitely non-nil
	nlTop                   // Could be either
)

func (n nilness) String() string {
	switch n {
	case nlBottom:
		return "⊥"
	case nlNil:
		return "nil"
	case nlNonNil:
		return "non-nil"
	case nlTop:
		return "⊤"
	default:
		return "?"
	}
}

// join computes the least upper bound of two nilness values.
func (n nilness) join(other nilness) nilness {
	if n == nlBottom {
		return other
	}
	if other == nlBottom {
		return n
	}
	if n == other {
		return n
	}
	return nlTop
}

// Equal reports whether two nilness values are equal.
func (n nilness) Equal(other nilness) bool { return n == other }

// =============================================================================
// Unified Lattice Value Interface
// =============================================================================

// latticeVal is the common interface for nilness and constBool lattice values.
// A nil latticeVal represents bottom (not yet computed) for any type.
type latticeVal interface {
	joinLattice(latticeVal) latticeVal
	isLatticeBottom() bool
}

func (n nilness) joinLattice(other latticeVal) latticeVal {
	if other == nil {
		return n
	}
	o, ok := other.(nilness)
	if !ok {
		return nlTop
	}
	return n.join(o)
}

func (n nilness) isLatticeBottom() bool { return n == nlBottom }

func (b constBool) joinLattice(other latticeVal) latticeVal {
	if other == nil {
		return b
	}
	o, ok := other.(constBool)
	if !ok {
		return boolTop
	}
	return b.join(o)
}

func (b constBool) isLatticeBottom() bool { return b == boolBottom }

// joinLatticeVals joins two lattice values, handling nil (bottom) on either side.
func joinLatticeVals(a, b latticeVal) latticeVal {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return a.joinLattice(b)
}

// =============================================================================
// Boolean Constant Lattice - for boolean-typed values
// =============================================================================

// constBool represents an abstract boolean constant.
// This is used for boolean-typed values (results of comparisons, etc.).
type constBool int

const (
	boolBottom constBool = iota // Not yet computed
	boolFalse                   // Definitely false
	boolTrue                    // Definitely true
	boolTop                     // Could be either (unknown)
)

func (b constBool) String() string {
	switch b {
	case boolBottom:
		return "⊥"
	case boolFalse:
		return "false"
	case boolTrue:
		return "true"
	case boolTop:
		return "⊤"
	default:
		return "?"
	}
}

// join computes the least upper bound of two boolean values.
func (b constBool) join(other constBool) constBool {
	if b == boolBottom {
		return other
	}
	if other == boolBottom {
		return b
	}
	if b == other {
		return b
	}
	return boolTop
}

// Equal reports whether two constBool values are equal.
func (b constBool) Equal(other constBool) bool { return b == other }

// negate returns the logical negation of a boolean constant.
func (b constBool) negate() constBool {
	switch b {
	case boolTrue:
		return boolFalse
	case boolFalse:
		return boolTrue
	default:
		return b // Bottom stays Bottom, Top stays Top
	}
}

// =============================================================================
// Synthetic Lattice - for tracking synthetic error creators
// =============================================================================

// synthetic tracks whether a value is created by a synthetic error creator
// (errors.New, fmt.Errorf). Uses OR-join: once any source is synthetic,
// the merged value is synthetic.
//
// Lattice: synthBottom < synthFalse < synthTrue (total order, join = max)
type synthetic int

const (
	synthBottom synthetic = iota // Not yet computed
	synthFalse                   // Not synthetic
	synthTrue                    // At least one source is synthetic
)

func (s synthetic) String() string {
	switch s {
	case synthBottom:
		return "⊥"
	case synthFalse:
		return "false"
	case synthTrue:
		return "true"
	default:
		return "?"
	}
}

// Equal reports whether two synthetic values are equal.
func (s synthetic) Equal(other synthetic) bool { return s == other }

// join computes the least upper bound of two synthetic values.
// Since synthetic is a total order, join is simply max.
func (s synthetic) join(other synthetic) synthetic {
	if s > other {
		return s
	}
	return other
}
