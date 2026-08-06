# SCCP Fail-Close Analyzer

A static analyzer that determines whether a Go function is **fail-close** with
respect to an RPC call's error return, using Sparse Conditional Constant
Propagation (SCCP) extended with assertion propagation, source tracking, and
control dependence analysis.

## Motivation

rpcshield enforces that services handle RPC errors correctly. When an outbound
RPC returns an error, the calling function should propagate that error (or a
derivative) to its own caller — this is called **fail-close** behavior. The
alternative, **fail-open**, silently discards the error and continues as if the
call succeeded, which can lead to data corruption, silent failures, or
security vulnerabilities.

Previous analyzers (SSA pattern-matching, AST heuristics) work well for simple
cases but struggle with complex control flow: multi-source error merging,
short-circuit boolean guards, named returns with deferred wrappers, and
assertion-based disambiguation. The SCCP analyzer handles all of these through
a principled lattice-based fixed-point computation.

## Formal Definition

A function *f* is **fail-close** for RPC call **Src** if and only if all four
properties hold:

### 1. Reachability

There exists at least one `return` statement reachable when Src's error is
non-nil.

### 2. Non-Suppression

On that return path, the returned error **RetErr** may be non-nil (it has not
been suppressed to `nil`).

### 3. Causal Attribution

**RetErr** is causally attributed to **Src**, written **Attr(r, Src)**, if at
least one of:

**(a) Data Attribution.** Src ∈ DataSources(RetErr), where DataSources(v) is
the set of error-producing call sites from which v is transitively derived
through copies, φ-nodes, and error-wrapping calls.

**(b) Control Attribution.** DataSources(RetErr) ∩ {Src} = ∅, AND there exists
a path *P* = [c₁, c₂, …, cₖ] in the inverse Control Dependence Graph from
*r*'s block to its transitive controllers, ordered innermost-first. Let *j* be
the smallest index such that cⱼ is a conditional branch on a nil-check of some
error value *v*, and *r*'s block is reachable on the non-nil (error) edge of
cⱼ. Then:

- If v = Src or Src ∈ DataSources(v): attribution holds.
- If DataSources(v) ∩ {Src} = ∅: attribution does **not** hold; the nearer
  source pre-empts Src's claim.

### 4. Inverse Plausibility

When Src is nil, there exists at least one reachable return whose error value
may be nil. This confirms Src actually controls the error outcome — without
it, the function would return non-nil errors regardless of Src.

## Four-Phase Algorithm

### Phase 1: Forward SCCP with Assertion Propagation

Runs SCCP [[Wegman & Zadeck, TOPLAS 1991]](https://doi.org/10.1145/103135.103136)
extended with assertion propagation [[Sakharov, SIGPLAN Notices 1994]](https://doi.org/10.1145/177965.177971)
to compute, in a single fixed-point pass:

| Domain | Lattice | Purpose |
|--------|---------|---------|
| **Nilness** | ⊥ → {nil, non-nil} → ⊤ | Tracks whether error values are nil, non-nil, or unknown |
| **Boolean constants** | ⊥ → {true, false} → ⊤ | Resolves branch conditions to prune infeasible paths |
| **Data sources** | Set of source IDs | Tracks which RPC calls produced each error value |
| **Synthetic flag** | ⊥ → {false, true} | Marks errors created by `errors.New` / `fmt.Errorf` |
| **Block executability** | {false, true} | Whether a block is reachable under current assumptions |
| **Assertion facts** | Map[Value → Nilness] | Per-block refined nilness from conditional guards |

**Scope restriction.** Before SCCP runs, we compute:

- **Preds(Src)** — transitive predecessors of Src's block in the CFG
- **Succs(Src)** — transitive successors of Src's block in the CFG
- **R = Preds(Src) ∪ Succs(Src) ∪ {srcBlock}** — only blocks in R participate

Blocks outside R cannot influence or be influenced by Src's error and are
excluded entirely.

**Predecessor path forcing.** At `if` terminators in Preds(Src) \ Succs(Src),
the condition is forced toward the successor that leads to Src, ensuring SCCP
follows the path that reaches Src rather than diverging.

**Assertion propagation.** When SCCP encounters `if err != nil`, it propagates
`err = NonNil` into the error branch and `err = Nil` into the success branch.
This handles patterns like:

```go
if err != nil {
    return "", err    // err is NonNil here (by assertion)
}
// err is Nil here (by assertion)
```

**Fixed-point convergence.** The algorithm uses two worklists (CFG blocks and
SSA instructions) and iterates until no lattice value changes. Monotonicity of
all lattice joins and finite lattice height guarantee termination.

### Phase 2: Non-Suppression Check

For each return in the function, determines whether the returned error may be
non-nil:

1. **Block-local assertion facts** — check if a conditional guard in the
   return's block proves nilness.
2. **Global lattice state** — use the SCCP-computed nilness.
3. **CDG fallback** — when nilness is Top (unknown), walk the inverse Control
   Dependence Graph to find a controlling condition that tests RetErr on its
   error branch.

In **lenient mode** (`MaybeAnalysisMode=true`), Top satisfies non-suppression
(may be non-nil). In **strict mode**, only NonNil satisfies.

### Phase 3: Causal Attribution

Proves the returned error is causally connected to Src:

1. **Exclusive data attribution** — DataSources(RetErr) = {Src}. Immediate
   pass.
2. **Ambiguous data attribution** — Src ∈ DataSources(RetErr) with other
   sources present. Passes if every other source has a Nil assertion fact at
   the return block.
3. **Control attribution** — walks the inverse CDG from the return block,
   finds the innermost nil-check, and checks whether it tests Src on the
   error branch. A different source on the error branch pre-empts Src.

### Phase 4: Inverse Plausibility

Re-runs SCCP with Src = Nil (all other sources unchanged). If at least one
reachable return may return nil, Src genuinely controls the error outcome.
If all reachable returns are non-nil regardless of Src, the function always
fails — Src is not the cause.

## Implementation

### File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `sccp_analyze.go` | ~2700 | Core four-phase algorithm |
| `lattice.go` | ~170 | Nilness, constBool, and synthetic lattice types with join semantics |
| `state.go` | ~120 | Per-value (`valState`) and per-block (`blkState`) abstract state |
| `sccp_facts.go` | ~100 | Per-block assertion fact maps with join/clone/equality |
| `nilcheck.go` | ~250 | Nil-check pattern detection, short-circuit AND/OR, attribution verdicts |
| `root_source.go` | ~90 | Immutable source registry (Src, OtherSources, synthetics) |
| `reachable.go` | ~80 | CFG reachability (Preds/Succs/R computation, feasible-edge BFS) |
| `sccp_metrics.go` | ~170 | Aggregate metrics with nil-safe increment methods |
| `options.go` | ~50 | Analysis configuration (mode, inverse, forward path, logging) |
| `simple.go` | ~350 | Legacy SSA analyzer (pre-SCCP) |

### Key Data Structures

**Analyzer state** holds the full context for one SCCP run:

```
valueState   map[Value → valState]       // Abstract state per SSA value
blockState   map[Block → blkState]       // Executability + assertion facts
cfgWorklist  []Block                     // Blocks to (re-)process
ssaWorklist  []Instruction               // Instructions to (re-)evaluate
feasibleEdges  map[Edge → bool]          // Reachable CFG edges
memState     map[Value → valState]       // Store→Load forwarding for named returns
sources      *rootSourceRegistry         // Immutable source registry
inverseCDG   *InverseCDG                 // Lazily built, shared across phases
```

**valState** captures the abstract state of a single SSA value:

```
lattice       latticeVal                 // Nilness or boolean constant
nilInfo       nilCheckInfo               // Which nil-checks this boolean represents
dataSources   sourceSet                  // Error-producing call sites
isSynthetic   synthetic                  // Created by errors.New / fmt.Errorf
```

### Configuration

| Option | Default | Effect |
|--------|---------|--------|
| `MaybeAnalysisMode` | `true` | Lenient: Top nilness satisfies non-suppression |
| `InverseCheck` | `false` | Enable Phase 4 (inverse plausibility) |
| `ForwardPathCheck` | `false` | Only consider returns reachable from Src via feasible edges |
| `OtherSources` | `nil` | Additional error-producing calls to track |
| `Metrics` | `nil` | Aggregate metrics collection |
| `Logger` | `nil` | Debug logging |

### Entry Points

```go
// Full API with options.
func AnalyzeCallBySCCPWithOptions(f *ssa.Function, call ssa.CallInstruction, opts SCCPOptions) (constant.Failness, error)

// Convenience wrapper with default options.
func AnalyzeCallBySCCP(f *ssa.Function, call ssa.CallInstruction) (constant.Failness, error)
```

## Shadow Mode Evaluation

The SCCP analyzer runs as a **shadow** alongside the existing SSA and AST
analyzers. Shadow results are logged and serialized for offline comparison
without affecting production output.

### Results (March 2026)

Evaluated across **211 services** with SCCP shadow data (979 services total,
768 had no outbound RPCs to analyze):

| Metric | Count | Percentage |
|--------|-------|------------|
| Total call sites analyzed | 8,361 | |
| **Matches** (SCCP = Final) | 8,022 | **95.9%** |
| **Mismatches** | 339 | 4.1% |

#### Mismatch Breakdown

| SCCP Result → Final Result | Count | Interpretation |
|---------------------------|-------|----------------|
| FailClose → FailOpen | 161 | SCCP more aggressive: sees fail-close where AST/SSA see fail-open |
| FailClose → FailUnknownIntended | 156 | Service owner override — SCCP correctly analyzes code, but result is overridden by annotation |
| FailUnknown → FailOpen | 11 | SCCP couldn't determine; final used AST heuristic |
| FailOpen → FailClose | 9 | SCCP more conservative: sees fail-open where final sees fail-close |
| FailOpen → FailUnknownIntended | 2 | |

#### Mismatches by Final Analyzer Source

| Source | Count |
|--------|-------|
| AST | 330 |
| SSA | 6 |
| LLM | 3 |

97.3% of mismatches are with the AST analyzer, which uses syntactic heuristics
rather than semantic analysis. The 9 FailOpen → FailClose cases (where SCCP is
more conservative) are the only potential false negatives and warrant manual
review.

#### Result Distributions

| | SCCP | Final |
|---|------|-------|
| FailClose | 7,567 (90.5%) | 7,259 (86.8%) |
| FailOpen | 783 (9.4%) | 944 (11.3%) |
| FailUnknown | 11 (0.1%) | 158 (1.9%) |

SCCP resolves 147 more call sites to a definite result (FailClose or FailOpen)
compared to the existing analyzers, reducing the FailUnknown rate from 1.9% to
0.1%.

## Test Coverage

Test coverage is **90.1%** (2006/2228 lines). Tests exercise:

- All lattice join semantics and edge cases
- All four analysis phases through 86+ permutation functions
- Forward-only, inverse, forward-path, strict, and lenient modes
- Nil-input error paths, metrics collection, and default API variants
- Named returns with deferred wrappers, short-circuit boolean guards,
  multi-source disambiguation, and CDG-based attribution

## References

1. M. N. Wegman and F. K. Zadeck, "Constant Propagation with Conditional
   Branches," *ACM TOPLAS* 13(2), pp. 181–210, 1991.
2. A. Sakharov, "Propagation of Constants and Assertions," *ACM SIGPLAN
   Notices* 29(3), pp. 3–6, 1994.
3. J. Ferrante, K. J. Ottenstein, and J. D. Warren, "The Program Dependence
   Graph and Its Use in Optimization," *ACM TOPLAS* 9(3), pp. 319–349, 1987.
