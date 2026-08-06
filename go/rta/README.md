# RTA: Rapid Type Analysis Framework for rpcshield

## Overview

This package provides a pluggable framework for Rapid Type Analysis (RTA) call
graph construction, designed to support multiple implementation strategies
(sequential, parallel, with and without method indexing) behind a unified
interface. RTA is used by rpcshield to build whole-program call graphs that
drive outbound RPC detection and service dependency analysis.

## Why a Custom RTA?

The standard `golang.org/x/tools/go/callgraph/rta` implementation is
single-threaded and performs O(C x I) interface-implementation checks where C is
the number of concrete types and I is the number of interface types. For large
Go monorepo services (10k+ concrete types, 5k+ interfaces), this becomes a
bottleneck:

- **Quadratic scaling**: Every new concrete type is checked against every known
  interface, and vice versa. The `types.Implements` check is expensive.
- **Single-threaded**: Cannot exploit multi-core machines during analysis.
- **No early pruning**: The standard implementation has no mechanism to skip
  obviously-incompatible type pairs.

This framework addresses all three issues through a family of RTA
implementations ("flavors") that progressively add optimizations.

## Architecture

```
rta/
├── lib.go              # Core interfaces: Result, RTA, AnalyzeOption, BaseMetrics
├── utils/              # Shared utilities (concurrent callgraph, fingerprinting, dedup)
├── srta/               # Sequential RTA baseline (this diff)
├── srta_kumo/          # Sequential + method indexing (next diff)
├── prta_naive/         # Parallel with global locks
├── prta_nonblocking/   # Parallel with lock-free callgraph
├── prta_kumo/          # Parallel + method indexing
├── prta_kumo_nonblocking/  # Parallel + method indexing + lock-free callgraph
└── rtatest/            # Shared correctness test helper
```

### Core Abstractions (`lib.go`)

- **`RTA` interface**: All flavors implement `Analyze(roots, buildCallGraph, opts...)`.
- **`Result` interface**: Uniform access to callgraph, reachable functions, and runtime types.
- **`BaseMetrics`**: Embeddable struct providing getter methods for all metric
  counters. Flavors embed this and populate the fields they support.
- **`WorkerCounters`**: Cache-line-padded per-worker counters that avoid false
  sharing in parallel implementations.
- **Optional metric interfaces**: Consumers use type assertions
  (`ImplementsCounter`, `TypeCounter`, `IndexStats`, etc.) to query
  flavor-specific metrics without coupling to concrete types.

### Utilities (`utils/`)

- **`fingerprint.go`**: Computes a 64-bit bitmask from a type's method set
  using CRC32 hashing. Enables O(1) rejection of non-implementing type pairs
  before the expensive `types.Implements` call.
- **`callgraph.go`**: `ConcurrentCallGraph` with per-node mutexes and atomic ID
  generation for lock-free node creation in parallel RTA.
- **`methodvalue.go`**: Cached method resolution via `MethodValueFast()` that
  avoids the SSA `Program.methodsMu` global lock contention.
- **`dedup.go`**: Edge deduplication with `PaddedCounter` and `WorkerCounter`
  types that use 64-byte cache-line padding to prevent false sharing.
- **`pmap.go`**: Thread-safe `TypeMap` with `LoadOrStore` semantics for
  concurrent type indexing.

### Sequential RTA (`srta/`)

A clean re-implementation of `golang.org/x/tools/go/callgraph/rta` with the
following additions:

- Implements the `rta.Result` interface (not the stdlib struct).
- Collects comprehensive metrics (implements call counts, call site breakdown,
  type counts) via `BaseMetrics`.
- Uses `utils.Fingerprint` for fast implements pre-filtering.
- Serves as the correctness baseline: all other flavors must produce identical
  callgraphs (verified by `rtatest.AssertMatchesStdlib`).

### Method-Indexed RTA (`srta_kumo/`)

The key insight behind the "Kumo" optimization is that the standard RTA
performs a brute-force cross-product between concrete types and interfaces. By
building two inverted indices — **MC** (method name → concrete types) and **MI**
(method name → interfaces) — we can dramatically reduce the number of
`types.Implements` checks.

**How it works:**

1. **MC Index**: When a new concrete type is discovered, each of its methods is
   registered in MC. Instead of checking against *all* known interfaces, we
   only check against interfaces that share at least one method name (looked up
   via MI).

2. **MI Index**: When a new interface is discovered, we pick *one* method to
   index it under in MI. The choice of which method to index determines how
   many false-positive candidates we scan.

3. **Method Selection Strategies**: The choice of indexed method is critical:
   - `RarestMethodByConcreteType` (default): Pick the method with the fewest
     concrete types in MC. This minimizes the candidate set when scanning from
     the implementation side.
   - `RarestMethodByInterfaceType`: Pick the method used by the fewest
     interfaces. This balances MI bucket sizes.
   - `RandomMethodStrategy`: Pick a random method (ablation baseline to
     measure the benefit of intelligent selection).

4. **Candidate Scanning**: When scanning for implementations of an interface,
   we pick the method with the smallest MC bucket and only check those concrete
   types — not all concrete types. An early-termination heuristic stops
   scanning methods once the smallest bucket found so far is already smaller
   than the number of remaining methods.

**`srta_kumo_random/`** is a thin wrapper that delegates to `srta_kumo` with
`RandomMethodStrategy` forced, providing an ablation baseline.

### Evaluation Dimensions

| Dimension | Baseline | Optimized |
|-----------|----------|-----------|
| Parallelism | srta (sequential) | prta_* (multi-worker) |
| Indexing | No index (brute-force) | Kumo MI/MC indices |
| Call Graph Sync | Global mutex | Per-node locks / lock-free CAS |
| Method Selection | N/A | Rarest-by-concrete / Rarest-by-interface / Random |

## Testing

Each flavor includes a correctness test (`impl_test.go`) that:
1. Loads the `rpcshielddemo` fixture's SSA program (`exampleapps/rpcshielddemo/`).
2. Runs both the stdlib RTA and the flavor under test.
3. Asserts identical node count, edge count, and graph neighbor hash
   (XOR of per-node SHA-256 fingerprints).

```bash
bazel test //rta/srta:go_default_test
```

See `evaluation_scripts/README.md` for the standalone OSS RTA benchmark
harness that runs these implementations against public GitHub repos.
