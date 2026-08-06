// Package syntheticfunc provides utilities for unwrapping synthetic functions in Go SSA.
//
// Synthetic functions are compiler-generated functions such as method wrappers,
// thunks, bounds, and generic instantiations that don't correspond directly to
// user-written code. This package helps resolve these synthetic functions back
// to their underlying concrete implementations.
package syntheticfunc

import (
	"fmt"
	"go/types"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// Unwrapper provides functionality to resolve synthetic functions to their concrete implementations.
// It maintains a cache of previously unwrapped functions to improve performance.
type Unwrapper struct {
	logger    *zap.SugaredLogger
	callgraph *callgraph.Graph

	// Cache containing unwrapped version of bound and thunk, as these are
	// expensive to compute.
	cache map[*ssa.Function]unwrapResult
}

// unwrapResult represents an entry in the unwrapping cache, containing either
// the successfully unwrapped function or an error from the unwrapping attempt.
type unwrapResult struct {
	fn  *ssa.Function
	err error
}

// Define constant error object to avoid allocation in some places

type unwrapError string

func (e unwrapError) Error() string {
	return string(e)
}

const failedToUnwrapError = unwrapError("Failed to unwrap function")

// NewUnwrapper creates a new Unwrapper instance with the provided logger.
func NewUnwrapper(logger *zap.SugaredLogger) *Unwrapper {
	return &Unwrapper{
		logger: logger,
	}
}

// WithCallgraph sets the callgraph for this unwrapper and returns the unwrapper for chaining.
func (u *Unwrapper) WithCallgraph(cg *callgraph.Graph, origin *ssa.Program) *Unwrapper {
	// We precompute the solution during callgraph registration and store it in
	// the cache. This results in an amortized cost of O(N) for N nodes in the
	// callgraph over M queries, as opposed to O(MN).
	u.populateCache(cg, origin)
	return u
}

// hashSSAFunc returns a hash for an SSA function.
// Two SSA functions have the same hash if the following are satisfied:
// 1. They share the same name
// 2. They share the same receiver type (with pointer stripped away)
// 3. They are in the same package
func (u *Unwrapper) hashSSAFunc(fn *ssa.Function) string {
	// We don't rely on fn.String() as a first line of attack for a couple reasons:
	// 1. It does not perform pointer receiver unwrapping. We need this definitionally.
	// 2. fn.String() internally calls fn.RelString(). The SSA package authors
	//    give no guarantee on the formatting of the output.
	//
	// The recvType, when formatted, contains the package name.

	recvTypes := u.getRecvTypes(fn)
	if len(recvTypes) > 0 {
		recvType := recvTypes[len(recvTypes)-1].String()
		return fmt.Sprintf("(%s) %s", recvType, fn.Name())
	}

	// There is no receiver type, we need to encode package info into the hash
	// to disambiguate.

	if fn.Pkg == nil {
		// Fallback to String(). Can't do better here.
		return fn.String()
	}
	return fmt.Sprintf("%s.%s", fn.Pkg, fn.Name())
}

// getRecvTypes returns all receiver types to compare against when matching
// synthetic functions to their concrete implementations.
// Cases:
//  1. Function could have recv type or not. Returns empty list
//  2. Function has a receiver
//     2.1 receiver is a pointer, return [ptr type, elem type]
//     2.2 reciver is not a pointer, return [elem type]
func (u *Unwrapper) getRecvTypes(fn *ssa.Function) []types.Type {
	if fn.Signature.Recv() == nil {
		return []types.Type{}
	}

	ty := fn.Signature.Recv().Type()
	base := []types.Type{ty}
	if ptrType, ok := ty.(*types.Pointer); ok {
		base = append(base, ptrType.Elem())
	}
	return base
}

// Helper function to populate cache for bound/thunk wrappers
func (u *Unwrapper) populateCache(cg *callgraph.Graph, origin *ssa.Program) {
	u.cache = make(map[*ssa.Function]unwrapResult)

	var synths []*ssa.Function

	// Stores a mapping from SSA function hash to an SSA function.
	// Why a custom hash?
	// 1. fn.Name() contains only the function name; no receiver or package
	//    info. The reverse map maintains the set of _functions to unwrap_. A
	//    collision here means that fewer functions will be unwrapped. For
	//    example, suppose we have two synthetic methods (impl1).Foo() and
	//    (impl2).Foo() that we wish to unwrap. If we use fn.Name(), only one of
	//    them will get unwrapped due to the collision.
	// 2. We cannot use *ssa.Function as the key as the unwrapped function and the
	//    wrapped function don't share the same function pointers. When we iterate
	//    through the callgraph, we need to know if the node is likely to be the
	//    unwrapped version of any of the synthetic functions we collected.
	reverse := make(map[string]*ssa.Function)

	for fn := range cg.Nodes {
		if fn.Synthetic != "" {
			if fn.Origin() != nil {
				// Function is generic
				u.cache[fn] = unwrapResult{
					fn:  fn.Origin(),
					err: nil,
				}
			} else {
				// Function is bound, thunk
				// Extract by iterating through callgraph
				synths = append(synths, fn)
				u.cache[fn] = unwrapResult{
					fn:  nil,
					err: failedToUnwrapError,
				}
				reverse[u.hashSSAFunc(fn)] = fn
			}
		}
	}

	for ofn := range cg.Nodes {
		if fn, ok := reverse[u.hashSSAFunc(ofn)]; ok {
			fnRecvTypes := u.getRecvTypes(fn)
			ofnRecvTypes := u.getRecvTypes(ofn)
			for _, fnRecvType := range fnRecvTypes {
				for _, ofnRecvType := range ofnRecvTypes {
					if fnRecvType == ofnRecvType && ofn.Pkg != nil && ofn.Pkg.Pkg != nil {
						u.cache[fn] = unwrapResult{
							fn:  ofn,
							err: nil,
						}
					}
				}
			}
		}
	}
}

// IsWrapped checks if a function is synthetic or wrapped.
func (u *Unwrapper) IsWrapped(fn *ssa.Function) bool {
	return fn.Synthetic != "" || fn.Pkg == nil || fn.Pkg.Pkg == nil
}

// TryUnwrap attempts to unwrap a synthetic function to its concrete implementation.
// Returns the unwrapped function as ssa.Value, or an error if unwrapping fails.
// For non-synthetic functions, returns the function itself.
func (u *Unwrapper) TryUnwrap(fn *ssa.Function) (ssa.Value, error) {
	var val ssa.Value
	var err error
	if u.callgraph == nil {
		val, err = u.unwrapFromBody(fn)
	} else {
		val, err = u.unwrapUsingCallgraph(fn)
	}

	return val, err
}

func (u *Unwrapper) unwrapFromBody(fn *ssa.Function) (ssa.Value, error) {
	if !u.IsWrapped(fn) {
		return fn, nil
	}

	// Recursively unwrap generic functions
	if fn.Origin() != nil {
		fn = fn.Origin()
		return u.unwrapFromBody(fn)
	}

	// Recursively unwrap wrapped functions

	// Heuristic: assume that second-to-last instruction of the entry block is
	// the call to the underlying function

	var callsite *ssa.Call

outer:
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch x := instr.(type) {
			case *ssa.Call:
				xName := x.Call.Value.Name()
				if x.Call.IsInvoke() {
					xName = x.Call.Method.Name()
				}
				if strings.Contains(fn.Name(), xName) {
					callsite = x
					break outer
				}
			}
		}
	}

	if callsite == nil {
		return nil, fmt.Errorf("could not find real func for %v because could not identify call site", fn)
	}

	staticCallee := callsite.Call.StaticCallee()
	if staticCallee == nil {
		// Dynamic dispatch, cannot unwrap further.
		return callsite, fmt.Errorf("Could not find real func for %v because the body of the wrapped function makes a dynamically dispatched call (%v) to the underlying function", fn.String(), callsite.Call.Value)
	}

	return u.unwrapFromBody(staticCallee)
}

func (u *Unwrapper) unwrapUsingCallgraph(fn *ssa.Function) (ssa.Value, error) {
	if !u.IsWrapped(fn) {
		return fn, nil
	}

	// Read solution off the cache
	if result, ok := u.cache[fn]; ok {
		return result.fn, result.err
	}

	return nil, fmt.Errorf("Failed to unwrap %v because it likely does not belong to the registered callgraph", fn)
}

// Unwrap attempts to unwrap a synthetic function to its concrete implementation.
// Returns the unwrapped function as *ssa.Function, or an error if unwrapping fails
// or if the unwrapped function has a nil package (indicating it's still synthetic).
func (u *Unwrapper) Unwrap(fn *ssa.Function) (*ssa.Function, error) {
	res, err := u.TryUnwrap(fn)
	if err != nil {
		return nil, err
	}

	realFn := res.(*ssa.Function)
	if realFn.Pkg != nil && realFn.Pkg.Pkg != nil {
		return realFn, nil
	}

	return nil, fmt.Errorf("The unwrapped function %s of the synthetic function %s has a nil package", realFn.String(), fn.String())
}
