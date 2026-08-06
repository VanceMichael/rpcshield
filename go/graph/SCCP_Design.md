# SCCP Algorithm Design for Interprocedural Call Chain Analysis

## High-Level Concept

This implementation applies **Sparse Conditional Constant Propagation (SCCP)** to analyze entire **call chains** in Go programs. The goal is to determine if a sequence of function calls is reachable given program constraints, and to prune impossible execution paths early.

**Core Problem**: Given a call chain A → B → C → D, can this entire sequence execute, or are there conditions that make parts of it unreachable?

**Solution Approach**: Extend classical SCCP with a dual lattice system that tracks both **values** and **types** as information flows through function calls, enabling precise resolution of Go's dynamic features. The lattice is tuple-aware to model SSA tuples (multi-value results) precisely.

## Key Innovations Beyond Classical SCCP

### 1. Interprocedural Extension
- **Classical SCCP**: Single-function analysis within a control-flow graph.
- **This Implementation**: Multi-function analysis across call boundaries.
- **Mechanics**: Parameter and free-var binding propagate lattice states from caller arguments to callee parameters, per link in the call chain.

### 2. Dual, Tuple-Aware Lattice System
- **Value Lattice**: `TopVal` → `ConstantVal` or `FunctionVal` → `BottomVal`.
- **Type Lattice**: `TopType` → `ConcreteType` → `BottomType`.
- **Combined Element**: `LatticeElement` holds a slice of `ValueTypePair` to represent tuples. Most SSA values are singletons; multi-result instructions (e.g., comma-ok type assertions) produce multiple `ValueTypePair` entries.
- **Meet Semantics**:
  - Value: `meet(Constant, Constant)` yields same constant else `BottomVal`; `meet(Function, Function)` yields same function else `BottomVal`; `Top` is identity; `Bottom` dominates.
  - Type: `meet(Concrete, Concrete)` yields same type else `BottomType`; `TopType` is identity; `BottomType` dominates.
  - Tuple length mismatch yields a single `ValueTypePair` bottom (value and type) to conservatively represent inconsistency.

### 3. Go-Specific Features
- **Interface Method Resolution** when receiver concrete type is known and non-interface.
- **Closure Analysis**: Track `ssa.MakeClosure` with bindings to propagate captured environments.
- **Type Assertion Modeling**: Handle both single-value and comma-ok assertions with precise success/failure modeling.
- **Phi Semantics**: Parallel read-then-write of phi nodes to respect SSA semantics when new executable edges become reachable.

## Algorithm Overview

### Lattice Structures

**Value Lattice**:
```
        ⊤ (Top)
       /          \
  ConstantVal  FunctionVal
       \          /
        ⊥ (Bottom)
```

**Type Lattice**:
```
    ⊤ (Top)
         |
   ConcreteType
         |
    ⊥ (Bottom)
```

`LatticeElement` = list of `(Value, Type)` pairs to represent tuples. For singletons, the list has one element.

### Analysis Flow

1. **Initialization**: For each function in the chain, initialize lattice for params, free vars, operands, and instruction results. Entry and recover blocks are marked reachable.
2. **Worklists**: Maintain both SSA and CFG worklists. Visit CFG items (blocks) and SSA values until both worklists drain.
3. **Phi Handling**: Evaluate all phi nodes in a block in a two-phase manner (read all, then write updated states). Revisit phi-only when new incoming edges are marked reachable.
4. **Instruction Modeling**: Evaluate instructions to refine lattice values. Non-modeled cases conservatively produce typed bottom.
5. **Control Flow Propagation**:
   - `if` with constant bool: propagate only the taken edge.
   - `if` with bottom: propagate both edges.
   - `if` with top: recorded for metrics; no edge added unless later refined.
   - `jump`: always propagate to successor.
6. **Call Resolution**: Attempt to resolve callee at each callsite; prune chain if resolved callee contradicts expected link.
7. **Environment Setup**: Map caller argument lattices to callee params; bind closure free vars when applicable.

## Instruction Semantics

Modeled instructions and key behaviors (see `constprop.go`):
- **Phi**: Meets values along executable incoming CFG edges; top is identity for meet.
- **BinOp**: Precise when operands are constants/functions. Handles comparisons, shifts, integer division/remainder (with zero checks), and defaults to `constant.BinaryOp`. Nil-comparisons support only `==`/`!=`. Mixed function/constant comparisons limited to nil equality.
- **UnOp**: All except channel receive (`<-`) can be constant-folded; channel receive yields bottom. Memory deref (`*x`) not modeled yet (conservative).
- **TypeAssert**: Detailed success/failure modeling. Returns either a typed value and `true`, or typed zero (incl. nil for nillable types) and `false`. Comma-ok assertions return a tuple `(value, ok)`.
- **Extract**: Extracts from tuples. Tuple-of-tuples is assumed impossible by Go SSA; if encountered, analysis panics intentionally to surface the corner case.
- **ChangeType/Convert**: Propagate the operand's value lattice and set the target concrete type.
- **MakeClosure**: Produces `FunctionVal` with closure binding info.
- **MakeInterface**: Forwards underlying lattice from the wrapped value.
- **MakeChan/MakeMap/MakeSlice/Alloc/Slice**: Create non-nil, typed unknown constants; `slice` yields bottom when base is a nil constant.
- **Default Fallback**: Yields typed bottom; if the result type is a tuple, returns tuple-typed bottom with one entry per result.

## Call Resolution Through Type Refinement

### Static Calls
If `call.Common().StaticCallee()` is present, use it directly (with closure bindings if `ssa.MakeClosure`).

### Interface Method Invokes
When `call.Common().IsInvoke()` is true:
- Get the receiver lattice type. If it is `ConcreteType` and not an interface, look up the concrete method via `prog.LookupMethod`.
- If the resolved method differs from the expected `link.Callee.Func`, the chain is deemed infeasible and pruned.
- Otherwise, if unresolved (receiver not concrete or is an interface), conservatively retain the expected callee.

### Indirect Calls (Function Values)
- If the function lattice is a `FunctionVal`:
  - For `*ssa.Function`: must equal `link.Callee.Func` or the chain is infeasible.
  - For `*ssa.MakeClosure`: must wrap the same `Fn` as `link.Callee.Func`; expose `Bindings` to bind callee free vars.
- Otherwise, if not a `FunctionVal`, conservatively retain the expected callee.

## Interprocedural Analysis Process

### Call Chain Processing Strategy
The analyzer processes each link in order:
1. Run SCCP within the caller function using current bindings.
2. If the callsite block is unreachable, the chain is infeasible and analysis returns early.
3. Resolve the callee per above. If a definite mismatch is detected, the chain is infeasible and analysis returns early.
4. Build the bindings for the next callee and proceed.

Special-case retention: designated edges (synthetic, etc.) may be marked as “do not prune”; their callsite blocks are force-marked reachable even if standard checks would prune them.

### Parameter and Free-Var Binding

Bindings for the next callee are constructed as follows:
- **Method Receiver (invoke)**: Map receiver lattice from callsite to callee's first param if and only if `types.AssignableTo(signatureReceiverType, callsiteReceiverType)` holds; otherwise bind all callee params to typed bottom.
- **Args → Params**: Number and type must match (`AssignableTo`); on mismatch, bind all params to typed bottom. On success, copy each argument's lattice to the corresponding parameter.
- **Free Vars**: By default conservative bottom. If the callsite function value is a `MakeClosure`, and the number of `FreeVars` matches `Bindings`, bind each callee free var from the captured binding's lattice.

### Multi-Level Pruning Strategy

- **Block Level**: Only analyze reachable basic blocks.
- **Control-Flow Level**: Constant conditions eliminate untaken branches; bottom conditions keep both branches.
- **Call Level**: Prune when:
  - Callsite is unreachable.
  - Resolved callee contradicts the expected callee.
  - Arguments/receiver incompatible with callee signature (binding falls back to typed bottom for safety).

## Type Assertion Effects (Detailed)

Given `x.(T)` (with or without comma-ok):
- If `x` is a constant nil of interface type: assertion fails → `(typedZero(T), false)`.
- If `x` has a concrete value/type (constant, function, or non-interface concrete type via lattice): check `types.AssignableTo(xType, T)`:
  - Success: `(value with type T, true)`.
  - Failure: `(typedZero(T), false)`.
- If `x` remains interface or otherwise uncertain (top/bottom with interface type): return typed bottoms conservatively when not enough information is available.

## Algorithm Properties

### Termination Guarantee
- **Monotonic Refinement**: Lattice values only become more precise.
- **Finite Height**: Value and Type lattices have height 3; each SSA value visits are bounded.
- **Fixed Point**: Analysis terminates when SSA and CFG worklists drain.

### Soundness
- **Conservative**: May keep infeasible paths feasible; never discards feasible paths.
- **Chain Pruning**: Only prunes when contradiction is provable (unreachable callsite, callee mismatch).

### Precision Benefits
- **Flow Sensitivity** across CFG edges.
- **Context Sensitivity** across call links via param/free-var bindings.
- **Type Awareness** enables devirtualization of method invokes and resolution of function pointers.

## Implementation Characteristics

### Complexity
- **Per-Function Time**: O(N + 3M) where N = number of basic blocks, M = number of SSA values.
- **Space**: O(N + M) for lattice states.
- **Chain Length**: Linear in call chain depth.

### Limitations
- **Memory Modeling**: Pointer loads/stores and global state are not modeled; many memory-sensitive ops bottom out.
- **Channel Receive**: Unary receive (`<-`) yields bottom.
- **Interface Receivers**: If a receiver remains interface, method dispatch is left unresolved.
- **Reflection/Dynamic**: Limited modeling of reflection and dynamic runtime features.
- **Tuple-of-Tuples**: Assumed impossible in Go SSA; encountering one intentionally panics to surface the corner case.


## Other Applications and Use Cases beyond rpcshield

### Security Analysis
- **Attack Path Validation**: Determine if malicious input can reach sensitive functions.
- **Call Chain Feasibility**: Verify whether exploit chains are actually executable.
- **Access Control Verification**: Check if authorization bypasses are possible.

### Performance Optimization
- **Dead Code Elimination**: Remove unreachable function calls and basic blocks.
- **Conditional Compilation**: Eliminate branches that are provably constant.
- **Call Devirtualization**: Replace interface calls with direct calls when type is known.

### Static Verification
- **API Contract Validation**: Verify parameter constraints across function boundaries.
- **Invariant Checking**: Ensure program invariants hold interprocedurally.
- **Configuration Validation**: Check if configuration values lead to valid execution paths.

## Summary

This SCCP implementation combines:
1. **Classical constant propagation** with proven semantics and correct phi handling.
2. **Interprocedural analysis** via precise environment propagation across call links.
3. **Go-specific extensions** for method devirtualization, closures, and type assertions.
4. **Multi-level pruning** based on reachability and callee resolution contradictions.
