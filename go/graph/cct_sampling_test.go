package graph

import (
	"fmt"
	"go/parser"
	"go/token"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestDefaultExplorationConfig(t *testing.T) {
	t.Parallel()
	cfg := defaultExplorationConfig()
	assert.Equal(t, int64(100), cfg.PermutationThreshold)
	assert.Equal(t, 3, cfg.MinCoreSize)
}

func TestNewExplorationState(t *testing.T) {
	t.Parallel()
	es := newExplorationState()
	assert.NotNil(t, es.sinkTrackers)
	assert.Equal(t, defaultExplorationConfig(), es.config)
}

// buildTestSSA creates an SSA program from a Go source string.
func buildTestSSA(t *testing.T, src string) *ssa.Program {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	conf := loader.Config{Fset: fset}
	conf.CreateFromFiles("testpkg", f)
	lprog, err := conf.Load()
	require.NoError(t, err)

	prog := ssautil.CreateProgram(lprog, ssa.InstantiateGenerics)
	prog.Build()
	return prog
}

// makeDummyFunctions creates distinct SSA functions for permutation tests.
func makeDummyFunctions(t *testing.T, count int) []*ssa.Function {
	t.Helper()
	// Build SSA with enough distinct functions.
	src := "package testpkg\n"
	for i := 0; i < count; i++ {
		src += fmt.Sprintf("func fn%d() {}\n", i)
	}
	prog := buildTestSSA(t, src)
	var fns []*ssa.Function
	for fn := range ssautil.AllFunctions(prog) {
		if fn.Synthetic == "" && fn.Name() != "init" {
			fns = append(fns, fn)
		}
	}
	// Sort for deterministic ordering.
	sort.Slice(fns, func(i, j int) bool {
		return fns[i].Name() < fns[j].Name()
	})
	require.GreaterOrEqual(t, len(fns), count)
	return fns[:count]
}

// makeIndirectCallSite builds SSA with an indirect call (f()) and returns the
// CallInstruction. Since the callee is a parameter (not a static function),
// StaticCallee() returns nil, making this an indirect/non-static callsite.
func makeIndirectCallSite(t *testing.T) ssa.CallInstruction {
	t.Helper()
	src := `package testpkg
func caller(f func()) { f() }
`
	prog := buildTestSSA(t, src)
	for fn := range ssautil.AllFunctions(prog) {
		if fn.Name() == "caller" {
			for _, block := range fn.Blocks {
				for _, instr := range block.Instrs {
					if call, ok := instr.(ssa.CallInstruction); ok {
						if call.Common().StaticCallee() == nil {
							return call
						}
					}
				}
			}
		}
	}
	t.Fatal("could not find indirect call site")
	return nil
}

// makeIndirectEdge creates a callgraph edge with a non-static (indirect) callsite.
func makeIndirectEdge(t *testing.T, caller, callee *ssa.Function, site ssa.CallInstruction) *callgraph.Edge {
	t.Helper()
	return &callgraph.Edge{
		Caller: &callgraph.Node{Func: caller},
		Site:   site,
		Callee: &callgraph.Node{Func: callee},
	}
}

func TestComputePathSetHash_OrderIndependent(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)

	path1 := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}, fns[2]: {}}
	path2 := map[*ssa.Function]struct{}{fns[2]: {}, fns[0]: {}, fns[1]: {}}
	path3 := map[*ssa.Function]struct{}{fns[1]: {}, fns[2]: {}, fns[0]: {}}

	h1 := computePathSetHash(path1)
	h2 := computePathSetHash(path2)
	h3 := computePathSetHash(path3)

	assert.Equal(t, h1, h2, "same set should produce same hash regardless of iteration order")
	assert.Equal(t, h1, h3)
}

func TestComputePathSetHash_DifferentSets(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)

	path1 := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}, fns[2]: {}}
	path2 := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}, fns[3]: {}}

	h1 := computePathSetHash(path1)
	h2 := computePathSetHash(path2)

	assert.NotEqual(t, h1, h2, "different sets should produce different hashes")
}

func TestComputePathSetHash_EmptySet(t *testing.T) {
	t.Parallel()
	path := map[*ssa.Function]struct{}{}
	assert.Equal(t, uint64(0), computePathSetHash(path))
}

func TestExtractPathSequence_EmptyStack(t *testing.T) {
	t.Parallel()
	seq := extractPathSequence(nil)
	assert.Empty(t, seq, "no edges means no indirect callees")
}

func TestExtractPathSequence_AllStatic(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)

	// Edges with no Site (treated as static) — should all be filtered out.
	stack := []*callgraph.Edge{
		{Caller: &callgraph.Node{Func: fns[0]}, Callee: &callgraph.Node{Func: fns[1]}},
		{Caller: &callgraph.Node{Func: fns[1]}, Callee: &callgraph.Node{Func: fns[2]}},
	}
	seq := extractPathSequence(stack)
	assert.Empty(t, seq, "static-only edges produce empty sequence")
}

func TestExtractPathSequence_WithIndirectCalls(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	site := makeIndirectCallSite(t)

	// Mix of static (no site) and indirect edges. Only indirect should appear.
	stack := []*callgraph.Edge{
		{Caller: &callgraph.Node{Func: fns[0]}, Callee: &callgraph.Node{Func: fns[1]}},            // static
		makeIndirectEdge(t, fns[1], fns[2], site),                                                 // indirect
		{Caller: &callgraph.Node{Func: fns[2]}, Callee: &callgraph.Node{Func: fns[3]}, Site: nil}, // static (nil site)
	}
	seq := extractPathSequence(stack)
	require.Len(t, seq, 1)
	assert.Equal(t, fns[2], seq[0], "only the indirect callee should appear")
}

func TestExtractPathSequence_MultipleIndirect(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	site := makeIndirectCallSite(t)

	stack := []*callgraph.Edge{
		makeIndirectEdge(t, fns[0], fns[1], site),
		makeIndirectEdge(t, fns[1], fns[2], site),
		makeIndirectEdge(t, fns[2], fns[3], site),
	}
	seq := extractPathSequence(stack)
	require.Len(t, seq, 3)
	assert.Equal(t, fns[1], seq[0])
	assert.Equal(t, fns[2], seq[1])
	assert.Equal(t, fns[3], seq[2])
}

func TestAccumulateCore_SameSequence(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	seq := []*ssa.Function{fns[0], fns[1], fns[2], fns[3]}
	coreSet := make(map[*ssa.Function]struct{})
	accumulateCore(seq, seq, coreSet)
	assert.Empty(t, coreSet, "identical sequences should add nothing")
}

func TestAccumulateCore_DifferentLengths(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	first := []*ssa.Function{fns[0], fns[1], fns[2]}
	second := []*ssa.Function{fns[0], fns[2], fns[1], fns[3]}
	coreSet := make(map[*ssa.Function]struct{})
	// Only compares up to min(len) = 3
	accumulateCore(first, second, coreSet)
	assert.Contains(t, coreSet, fns[1])
	assert.Contains(t, coreSet, fns[2])
	assert.NotContains(t, coreSet, fns[0])
}

func TestAccumulateCore_TwoSwapped(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	first := []*ssa.Function{fns[0], fns[1], fns[2], fns[3]}
	second := []*ssa.Function{fns[0], fns[2], fns[1], fns[3]}
	coreSet := make(map[*ssa.Function]struct{})
	accumulateCore(first, second, coreSet)
	assert.Len(t, coreSet, 2, "only 2 swapped positions")
	assert.Contains(t, coreSet, fns[1])
	assert.Contains(t, coreSet, fns[2])
}

func TestAccumulateCore_Incremental(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 5)
	first := []*ssa.Function{fns[0], fns[1], fns[2], fns[3], fns[4]}
	coreSet := make(map[*ssa.Function]struct{})

	// 2nd observation: only fns[1] and fns[2] swap → core has 2
	obs2 := []*ssa.Function{fns[0], fns[2], fns[1], fns[3], fns[4]}
	accumulateCore(first, obs2, coreSet)
	assert.Len(t, coreSet, 2)

	// 3rd observation: fns[3] and fns[4] also swap → core grows to 4
	obs3 := []*ssa.Function{fns[0], fns[1], fns[2], fns[4], fns[3]}
	accumulateCore(first, obs3, coreSet)
	assert.Len(t, coreSet, 4)
	assert.Contains(t, coreSet, fns[1])
	assert.Contains(t, coreSet, fns[2])
	assert.Contains(t, coreSet, fns[3])
	assert.Contains(t, coreSet, fns[4])
}

func TestRegisterCommutativeGroup(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)
	es := newExplorationState()

	es.registerCommutativeGroup(fns)
	require.Len(t, es.commutativeGroups, 1)

	group := es.commutativeGroups[0]
	assert.Len(t, group.members, 3)

	// Verify ranks are in sorted-by-name order.
	names := make([]string, 3)
	for fn, rank := range group.members {
		names[rank] = fn.String()
	}
	for i := 1; i < len(names); i++ {
		assert.Less(t, names[i-1], names[i], "ranks should follow sorted name order")
	}
}

func TestShouldSkipPermutation_NotInGroup(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 5)
	es := newExplorationState()
	es.registerCommutativeGroup(fns[:3]) // fns[0..2] in group

	// Visiting fns[4] (not in any group) should not be skipped.
	path := map[*ssa.Function]struct{}{fns[0]: {}}
	assert.False(t, es.shouldSkipPermutation(path, fns[4]))
}

func TestShouldSkipPermutation_CanonicalOrder(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	es := newExplorationState()

	// Register group with fns[0..2]. Ranks determined by sorted name.
	es.registerCommutativeGroup([]*ssa.Function{fns[0], fns[1], fns[2]})
	group := es.commutativeGroups[0]

	// Determine who has the lowest and highest rank.
	var lowFn, highFn *ssa.Function
	for fn, rank := range group.members {
		if rank == 0 {
			lowFn = fn
		}
		if rank == 2 {
			highFn = fn
		}
	}
	require.NotNil(t, lowFn)
	require.NotNil(t, highFn)

	// highFn already on path: visiting lowFn should be skipped (violates ascending order)
	path := map[*ssa.Function]struct{}{highFn: {}}
	assert.True(t, es.shouldSkipPermutation(path, lowFn))

	// lowFn already on path: visiting highFn should NOT be skipped (ascending is OK)
	path2 := map[*ssa.Function]struct{}{lowFn: {}}
	assert.False(t, es.shouldSkipPermutation(path2, highFn))
}

func TestShouldSkipPermutation_EmptyPath(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)
	es := newExplorationState()
	es.registerCommutativeGroup(fns)

	// Empty path: visiting any group member should be fine.
	path := map[*ssa.Function]struct{}{}
	for _, fn := range fns {
		assert.False(t, es.shouldSkipPermutation(path, fn))
	}
}

func TestTrackSinkPath_BelowThreshold(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	cfg := defaultExplorationConfig()
	cfg.PermutationThreshold = 5
	es := newExplorationStateWithConfig(cfg)

	path := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}}
	// Track 4 times (below threshold of 5): should NOT detect group.
	for i := 0; i < 4; i++ {
		es.trackSinkPath(fns[3], path, nil)
	}
	assert.Empty(t, es.commutativeGroups)
}

func TestTrackSinkPath_DisabledThreshold(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)
	cfg := defaultExplorationConfig()
	cfg.PermutationThreshold = 0
	es := newExplorationStateWithConfig(cfg)

	path := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}}
	for i := 0; i < 100; i++ {
		es.trackSinkPath(fns[2], path, nil)
	}
	assert.Empty(t, es.commutativeGroups, "disabled threshold should never trigger detection")
}

func TestTrackSinkPath_AtThreshold_WithPermutation(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 5)
	cfg := defaultExplorationConfig()
	cfg.PermutationThreshold = 4
	es := newExplorationStateWithConfig(cfg)
	site := makeIndirectCallSite(t)

	sinkFn := fns[4]

	path := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}, fns[2]: {}, fns[3]: {}}

	// 1st: fns[1], fns[2], fns[3] — records firstSeq
	stack1 := []*callgraph.Edge{
		makeIndirectEdge(t, fns[0], fns[1], site),
		makeIndirectEdge(t, fns[1], fns[2], site),
		makeIndirectEdge(t, fns[2], fns[3], site),
		{Caller: &callgraph.Node{Func: fns[3]}, Callee: nil},
	}
	es.trackSinkPath(sinkFn, path, stack1)
	assert.Empty(t, es.commutativeGroups)

	// 2nd: swap fns[1] and fns[2] only — coreSet grows to {fns[1], fns[2]} (only 2)
	stack2 := []*callgraph.Edge{
		makeIndirectEdge(t, fns[0], fns[2], site),
		makeIndirectEdge(t, fns[2], fns[1], site),
		makeIndirectEdge(t, fns[1], fns[3], site),
		{Caller: &callgraph.Node{Func: fns[3]}, Callee: nil},
	}
	es.trackSinkPath(sinkFn, path, stack2)
	assert.Empty(t, es.commutativeGroups)

	// 3rd: swap fns[2] and fns[3] — coreSet grows to {fns[1], fns[2], fns[3]} (3!)
	stack3 := []*callgraph.Edge{
		makeIndirectEdge(t, fns[0], fns[1], site),
		makeIndirectEdge(t, fns[1], fns[3], site),
		makeIndirectEdge(t, fns[3], fns[2], site),
		{Caller: &callgraph.Node{Func: fns[2]}, Callee: nil},
	}
	es.trackSinkPath(sinkFn, path, stack3)
	assert.Empty(t, es.commutativeGroups, "below threshold count, no trigger yet")

	// 4th (=threshold): same ordering as 1st — no NEW diffs, but core already has 3
	es.trackSinkPath(sinkFn, path, stack1)
	assert.NotEmpty(t, es.commutativeGroups, "should detect permutation at threshold with incrementally built core")
}

func TestTrackSinkPath_PerSinkScope(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	cfg := defaultExplorationConfig()
	cfg.PermutationThreshold = 3
	es := newExplorationStateWithConfig(cfg)

	path := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}}

	// Track 2 times on sink fns[2], 2 times on sink fns[3]
	// Neither should reach threshold of 3 since they're separate sinks
	es.trackSinkPath(fns[2], path, nil)
	es.trackSinkPath(fns[2], path, nil)
	es.trackSinkPath(fns[3], path, nil)
	es.trackSinkPath(fns[3], path, nil)

	assert.Empty(t, es.commutativeGroups, "counts should be per-sink, not aggregated")
}

func TestCommutativeGroup_ShouldSkip(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)

	group := &commutativeGroup{
		members: map[*ssa.Function]int{
			fns[0]: 0,
			fns[1]: 1,
			fns[2]: 2,
		},
	}

	// fns[2] (rank 2) on path, visiting fns[0] (rank 0): skip (higher rank already visited)
	assert.True(t, group.shouldSkip(
		map[*ssa.Function]struct{}{fns[2]: {}},
		fns[0],
	))

	// fns[0] (rank 0) on path, visiting fns[2] (rank 2): don't skip (ascending)
	assert.False(t, group.shouldSkip(
		map[*ssa.Function]struct{}{fns[0]: {}},
		fns[2],
	))

	// fns[3] (not in group) on path, visiting fns[1] (in group): don't skip
	assert.False(t, group.shouldSkip(
		map[*ssa.Function]struct{}{fns[3]: {}},
		fns[1],
	))

	// fns[3] (not in group): never skip
	assert.False(t, group.shouldSkip(
		map[*ssa.Function]struct{}{fns[2]: {}},
		fns[3],
	))
}

func TestShouldSkipConvergence_BelowThreshold(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)
	cfg := defaultExplorationConfig()
	cfg.MaxFunctionVisits = 5
	es := newExplorationStateWithConfig(cfg)

	// Visit fn 4 times (below threshold of 5): should never skip.
	for i := 0; i < 4; i++ {
		assert.False(t, es.ShouldSkipConvergence(fns[0]))
	}
	assert.Empty(t, es.GetConvergencePoints())
	assert.Equal(t, int64(0), es.ConvergenceSkips())
}

func TestShouldSkipConvergence_AtThreshold(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)
	cfg := defaultExplorationConfig()
	cfg.MaxFunctionVisits = 3
	es := newExplorationStateWithConfig(cfg)

	// Visit 3 times: threshold is 3, so the 4th visit (count > threshold) marks convergence.
	for i := 0; i < 3; i++ {
		assert.False(t, es.ShouldSkipConvergence(fns[0]))
	}
	// 4th visit: count becomes 4 > 3, marks as convergence but doesn't skip this visit.
	assert.False(t, es.ShouldSkipConvergence(fns[0]))
	assert.Len(t, es.GetConvergencePoints(), 1)

	// 5th visit: should skip because already marked.
	assert.True(t, es.ShouldSkipConvergence(fns[0]))
	assert.Equal(t, int64(1), es.ConvergenceSkips())
}

func TestShouldSkipConvergence_AlreadyMarked(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)
	cfg := defaultExplorationConfig()
	cfg.MaxFunctionVisits = 1
	es := newExplorationStateWithConfig(cfg)

	// 1st visit: count=1, not > 1, no convergence.
	assert.False(t, es.ShouldSkipConvergence(fns[0]))
	// 2nd visit: count=2 > 1, marks as convergence, doesn't skip yet.
	assert.False(t, es.ShouldSkipConvergence(fns[0]))
	// 3rd+: should skip.
	assert.True(t, es.ShouldSkipConvergence(fns[0]))
	assert.True(t, es.ShouldSkipConvergence(fns[0]))
	assert.Equal(t, int64(2), es.ConvergenceSkips())
}

func TestConvergenceSkips(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)
	cfg := defaultExplorationConfig()
	cfg.MaxFunctionVisits = 1
	es := newExplorationStateWithConfig(cfg)

	// Mark fns[0] as convergence point.
	es.ShouldSkipConvergence(fns[0]) // count=1
	es.ShouldSkipConvergence(fns[0]) // count=2 > 1, marked

	// Now skip twice.
	es.ShouldSkipConvergence(fns[0])
	es.ShouldSkipConvergence(fns[0])
	assert.Equal(t, int64(2), es.ConvergenceSkips())

	// Different function should not be affected.
	assert.False(t, es.ShouldSkipConvergence(fns[1]))
}

func TestGetConvergencePoints(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	cfg := defaultExplorationConfig()
	cfg.MaxFunctionVisits = 1
	es := newExplorationStateWithConfig(cfg)

	// Mark two functions as convergence points.
	es.ShouldSkipConvergence(fns[0])
	es.ShouldSkipConvergence(fns[0]) // marks fns[0]
	es.ShouldSkipConvergence(fns[1])
	es.ShouldSkipConvergence(fns[1]) // marks fns[1]

	points := es.GetConvergencePoints()
	assert.Len(t, points, 2)
}

func TestShouldSkipConvergence_Disabled(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 2)
	cfg := defaultExplorationConfig()
	cfg.MaxFunctionVisits = 0 // disabled
	es := newExplorationStateWithConfig(cfg)

	// Should never skip when convergence detection is disabled.
	for i := 0; i < 100; i++ {
		assert.False(t, es.ShouldSkipConvergence(fns[0]))
	}
	assert.Empty(t, es.GetConvergencePoints())
	assert.Equal(t, int64(0), es.ConvergenceSkips())
}

func TestAccumulateCore_CurrentShorterThanFirst(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	first := []*ssa.Function{fns[0], fns[1], fns[2], fns[3]}
	current := []*ssa.Function{fns[0], fns[2]} // shorter, with diff at position 1
	coreSet := make(map[*ssa.Function]struct{})
	accumulateCore(first, current, coreSet)
	// Only compares up to min(len) = 2; position 1 differs (fns[1] vs fns[2])
	assert.Contains(t, coreSet, fns[1])
	assert.Contains(t, coreSet, fns[2])
	assert.NotContains(t, coreSet, fns[0], "position 0 matches")
	assert.NotContains(t, coreSet, fns[3], "beyond min length")
}

func TestGetDiagnostics(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 3)
	cfg := defaultExplorationConfig()
	cfg.PermutationThreshold = 2
	cfg.MinCoreSize = 0
	es := newExplorationStateWithConfig(cfg)

	sinkFn := fns[2]
	path := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}}

	// Track paths to accumulate stats.
	es.trackSinkPath(sinkFn, path, nil)
	es.trackSinkPath(sinkFn, path, nil)

	diags := es.GetDiagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, sinkFn.String(), diags[0].SinkName)
	assert.Equal(t, int64(2), diags[0].TotalPaths)
	assert.Equal(t, int64(1), diags[0].UniqueSets)
	assert.Equal(t, int64(2), diags[0].MaxPerSet)
}

func TestPermutationThresholdHitPerSink(t *testing.T) {
	t.Parallel()
	fns := makeDummyFunctions(t, 4)
	cfg := defaultExplorationConfig()
	cfg.PermutationThreshold = 2
	cfg.MinCoreSize = 0
	es := newExplorationStateWithConfig(cfg)

	path := map[*ssa.Function]struct{}{fns[0]: {}, fns[1]: {}}

	// Track sink fns[2] twice (hits threshold), fns[3] once (does not).
	es.trackSinkPath(fns[2], path, nil)
	es.trackSinkPath(fns[2], path, nil)
	es.trackSinkPath(fns[3], path, nil)

	result := es.PermutationThresholdHitPerSink(nil)
	assert.True(t, result[fns[2].String()], "should hit threshold for fns[2]")
	assert.False(t, result[fns[3].String()], "should not hit threshold for fns[3]")

	// Test with custom names.
	names := map[*ssa.Function]string{fns[2]: "custom-sink"}
	result2 := es.PermutationThresholdHitPerSink(names)
	assert.True(t, result2["custom-sink"])
}
