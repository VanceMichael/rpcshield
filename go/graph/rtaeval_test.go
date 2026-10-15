package graph

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/mapper"
	"github.com/uber-research/rpcshield/go/loader"
	"github.com/uber-research/rpcshield/go/rta"
	"go.uber.org/zap"
	"golang.org/x/tools/go/callgraph"
	rtapkg "golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"golang.org/x/tools/go/types/typeutil"
)

func TestDefaultSerialGraphConfig(t *testing.T) {
	cfg := DefaultSerialGraphConfig()
	assert.Equal(t, FlavorStdlib, cfg.RTAFlavor)
	assert.Equal(t, 1, cfg.RTANumWorkers)
}

func TestDefaultParallelGraphConfig(t *testing.T) {
	cfg := DefaultParallelGraphConfig()
	assert.Equal(t, FlavorPRTAKumoNonblocking, cfg.RTAFlavor)
	assert.Equal(t, runtime.GOMAXPROCS(0), cfg.RTANumWorkers)
}

func TestRTAFlavorString(t *testing.T) {
	tests := []struct {
		flavor RTAFlavor
		want   string
	}{
		{FlavorSRTA, "srta"},
		{FlavorSRTAOpt, "srta_opt"},
		{FlavorSRTAStruct, "srta_struct"},
		{FlavorSRTAKumo, "srta_kumo"},
		{FlavorSRTAKumoRandom, "srta_kumo_random"},
		{FlavorPRTANaive, "prta_naive"},
		{FlavorPRTANonblocking, "prta_nonblocking"},
		{FlavorPRTAKumo, "prta_kumo"},
		{FlavorPRTAKumoNonblocking, "prta_kumo_nonblocking"},
		{FlavorStdlib, "stdlib"},
		{FlavorCHA, "cha"},
		{RTAFlavor(999), "unknown(999)"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.flavor.String())
	}
}

func TestRTAFlavorIsSequential(t *testing.T) {
	sequential := []RTAFlavor{FlavorSRTA, FlavorSRTAOpt, FlavorSRTAStruct, FlavorSRTAKumo, FlavorSRTAKumoRandom, FlavorStdlib, FlavorCHA}
	parallel := []RTAFlavor{FlavorPRTANaive, FlavorPRTANonblocking, FlavorPRTAKumo, FlavorPRTAKumoNonblocking}

	for _, f := range sequential {
		assert.True(t, f.IsSequential(), "%s should be sequential", f)
	}
	for _, f := range parallel {
		assert.False(t, f.IsSequential(), "%s should be parallel", f)
	}
}

func TestRTAFlavorNewImpl(t *testing.T) {
	// Custom flavors should return non-nil RTA implementations.
	customFlavors := []RTAFlavor{
		FlavorSRTA, FlavorSRTAOpt, FlavorSRTAStruct, FlavorSRTAKumo, FlavorSRTAKumoRandom,
		FlavorPRTANaive, FlavorPRTANonblocking, FlavorPRTAKumo, FlavorPRTAKumoNonblocking,
	}
	for _, f := range customFlavors {
		impl, _ := f.NewImpl()
		assert.NotNil(t, impl, "%s should return non-nil impl", f)
	}

	// Stdlib and CHA are handled separately — NewImpl returns nil.
	for _, f := range []RTAFlavor{FlavorStdlib, FlavorCHA} {
		impl, opts := f.NewImpl()
		assert.Nil(t, impl, "%s should return nil impl", f)
		assert.Nil(t, opts, "%s should return nil opts", f)
	}

	// SRTAKumoRandom should have extra options.
	_, opts := FlavorSRTAKumoRandom.NewImpl()
	assert.NotEmpty(t, opts, "srta_kumo_random should have extra options")
}

func TestParseRTAFlavor(t *testing.T) {
	tests := []struct {
		input   string
		want    RTAFlavor
		wantErr bool
	}{
		{"srta", FlavorSRTA, false},
		{"stdlib", FlavorStdlib, false},
		{"prta_kumo_nonblocking", FlavorPRTAKumoNonblocking, false},
		{" stdlib ", FlavorStdlib, false},
		{"invalid", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseRTAFlavor(tt.input)
		if tt.wantErr {
			assert.Error(t, err, "ParseRTAFlavor(%q)", tt.input)
		} else {
			require.NoError(t, err, "ParseRTAFlavor(%q)", tt.input)
			assert.Equal(t, tt.want, got)
		}
	}
}

func TestParseRTAFlavors(t *testing.T) {
	// Empty or "all" returns all flavors.
	all, err := ParseRTAFlavors("")
	require.NoError(t, err)
	assert.Equal(t, AllFlavors, all)

	all, err = ParseRTAFlavors("all")
	require.NoError(t, err)
	assert.Equal(t, AllFlavors, all)

	// Comma-separated list.
	flavors, err := ParseRTAFlavors("srta,stdlib")
	require.NoError(t, err)
	assert.Equal(t, []RTAFlavor{FlavorSRTA, FlavorStdlib}, flavors)

	// Whitespace handling.
	flavors, err = ParseRTAFlavors(" srta , stdlib ")
	require.NoError(t, err)
	assert.Equal(t, []RTAFlavor{FlavorSRTA, FlavorStdlib}, flavors)

	// Invalid flavor.
	_, err = ParseRTAFlavors("srta,bogus")
	assert.Error(t, err)

	// Only commas (empty parts) returns all.
	all, err = ParseRTAFlavors(",,,")
	require.NoError(t, err)
	assert.Equal(t, AllFlavors, all)
}

func TestParseRTANumWorkers(t *testing.T) {
	// Empty returns default.
	workers, err := ParseRTANumWorkers("")
	require.NoError(t, err)
	assert.Equal(t, []int{64}, workers)

	// Single value.
	workers, err = ParseRTANumWorkers("8")
	require.NoError(t, err)
	assert.Equal(t, []int{8}, workers)

	// Multiple values.
	workers, err = ParseRTANumWorkers("1,4,16")
	require.NoError(t, err)
	assert.Equal(t, []int{1, 4, 16}, workers)

	// Whitespace.
	workers, err = ParseRTANumWorkers(" 2 , 8 ")
	require.NoError(t, err)
	assert.Equal(t, []int{2, 8}, workers)

	// Invalid: non-number.
	_, err = ParseRTANumWorkers("abc")
	assert.Error(t, err)

	// Invalid: zero.
	_, err = ParseRTANumWorkers("0")
	assert.Error(t, err)

	// Invalid: negative.
	_, err = ParseRTANumWorkers("-1")
	assert.Error(t, err)

	// Only commas returns default.
	workers, err = ParseRTANumWorkers(",,,")
	require.NoError(t, err)
	assert.Equal(t, []int{64}, workers)
}

func TestEnsureStdlibFirst(t *testing.T) {
	// Stdlib not present — prepended.
	result := ensureStdlibFirst([]RTAFlavor{FlavorSRTA, FlavorPRTANaive})
	assert.Equal(t, FlavorStdlib, result[0])
	assert.Len(t, result, 3)

	// Stdlib already first — no duplication.
	result = ensureStdlibFirst([]RTAFlavor{FlavorStdlib, FlavorSRTA})
	assert.Equal(t, FlavorStdlib, result[0])
	assert.Len(t, result, 2)

	// Stdlib in middle — moved to front, no duplication.
	result = ensureStdlibFirst([]RTAFlavor{FlavorSRTA, FlavorStdlib, FlavorPRTANaive})
	assert.Equal(t, FlavorStdlib, result[0])
	assert.Len(t, result, 3)
	stdlibCount := 0
	for _, f := range result {
		if f == FlavorStdlib {
			stdlibCount++
		}
	}
	assert.Equal(t, 1, stdlibCount)

	// Empty input.
	result = ensureStdlibFirst([]RTAFlavor{})
	assert.Equal(t, []RTAFlavor{FlavorStdlib}, result)
}

func TestCompareCallGraphs(t *testing.T) {
	logger := zap.NewNop().Sugar()

	// Both nil — isomorphic.
	iso, diff := CompareCallGraphs(nil, nil, logger)
	assert.True(t, iso)
	assert.Empty(t, diff)

	// One nil — not isomorphic.
	cg := &callgraph.Graph{Nodes: map[*ssa.Function]*callgraph.Node{}}
	iso, diff = CompareCallGraphs(cg, nil, logger)
	assert.False(t, iso)
	assert.Contains(t, diff, "nil")

	iso, diff = CompareCallGraphs(nil, cg, logger)
	assert.False(t, iso)
	assert.Contains(t, diff, "nil")

	// Identical empty graphs.
	cg1 := &callgraph.Graph{Nodes: map[*ssa.Function]*callgraph.Node{}}
	cg2 := &callgraph.Graph{Nodes: map[*ssa.Function]*callgraph.Node{}}
	iso, diff = CompareCallGraphs(cg1, cg2, logger)
	assert.True(t, iso)
	assert.Empty(t, diff)

	// Different node counts.
	fn1 := &ssa.Function{}
	cg1 = &callgraph.Graph{Nodes: map[*ssa.Function]*callgraph.Node{
		fn1: {Func: fn1},
	}}
	cg2 = &callgraph.Graph{Nodes: map[*ssa.Function]*callgraph.Node{}}
	iso, diff = CompareCallGraphs(cg1, cg2, logger)
	assert.False(t, iso)
	assert.Contains(t, diff, "node count mismatch")
}

// mockRTAResult implements all rta metric interfaces for testing collectRTAMetrics.
type mockRTAResult struct {
	cg *callgraph.Graph
}

func (m *mockRTAResult) GetCallGraph() *callgraph.Graph                           { return m.cg }
func (m *mockRTAResult) GetReachable() map[*ssa.Function]struct{ AddrTaken bool } { return nil }
func (m *mockRTAResult) GetRuntimeTypes() typeutil.Map                            { return typeutil.Map{} }
func (m *mockRTAResult) Materialize() *rtapkg.Result                              { return nil }
func (m *mockRTAResult) ImplementsCallCount() int                                 { return 100 }
func (m *mockRTAResult) NumConcreteTypes() int                                    { return 200 }
func (m *mockRTAResult) NumInterfaceTypes() int                                   { return 50 }
func (m *mockRTAResult) ImplementsSuccessCount() int                              { return 80 }
func (m *mockRTAResult) ImplementsFailCount() int                                 { return 20 }
func (m *mockRTAResult) ChecksFromInterfaces() int                                { return 60 }
func (m *mockRTAResult) ChecksFromImplementations() int                           { return 40 }
func (m *mockRTAResult) FailsFromInterfaces() int                                 { return 5 }
func (m *mockRTAResult) SuccessFromInterfaces() int                               { return 55 }
func (m *mockRTAResult) FailsFromImplementations() int                            { return 15 }
func (m *mockRTAResult) SuccessFromImplementations() int                          { return 25 }
func (m *mockRTAResult) StaticCallSites() int                                     { return 500 }
func (m *mockRTAResult) IndirectCallSites() int                                   { return 30 }
func (m *mockRTAResult) InvokeCallSites() int                                     { return 70 }
func (m *mockRTAResult) TotalCallSites() int                                      { return 600 }
func (m *mockRTAResult) PreDedupDurationMs() float64                              { return 10.5 }
func (m *mockRTAResult) PostDedupDurationMs() float64                             { return 12.0 }
func (m *mockRTAResult) PreDedupHeapDeltaMB() float64                             { return 100.0 }
func (m *mockRTAResult) PostDedupHeapDeltaMB() float64                            { return 95.0 }
func (m *mockRTAResult) DuplicatedEdgeCount() int                                 { return 42 }
func (m *mockRTAResult) MCBucketSizePercentiles() rta.Percentiles {
	return rta.Percentiles{P50: 1, P90: 5, P95: 10, P99: 20, P100: 50}
}
func (m *mockRTAResult) MIBucketSizePercentiles() rta.Percentiles {
	return rta.Percentiles{P50: 2, P90: 4, P95: 6, P99: 8, P100: 10}
}
func (m *mockRTAResult) InterfaceMethodCountBuckets() [8]float64 {
	return [8]float64{0.1, 0.2, 0.15, 0.1, 0.05, 0.03, 0.02, 0.35}
}
func (m *mockRTAResult) MethodsPerConcretePercentiles() rta.Percentiles {
	return rta.Percentiles{P50: 3, P90: 0, P95: 7, P99: 12, P100: 25}
}
func (m *mockRTAResult) MethodsPerInterfacePercentiles() rta.Percentiles {
	return rta.Percentiles{P50: 1, P90: 0, P95: 3, P99: 5, P100: 8}
}
func (m *mockRTAResult) ConcreteMethodCountBuckets() [8]float64 {
	return [8]float64{0.4, 0.2, 0.1, 0.1, 0.05, 0.05, 0.05, 0.05}
}

func TestCollectRTAMetrics(t *testing.T) {
	logger := zap.NewNop().Sugar()
	b := NewBuilder(logger, nil, nil, nil, mapper.SSAASTMap{}, mapper.SignatureASTMap{}, false, nil, nil, nil, false, DefaultSerialGraphConfig())
	bi := b.(*builderImpl)

	mock := &mockRTAResult{}
	bi.collectRTAMetrics(mock, "test_flavor", 4)

	// ImplementsCounter
	assert.Equal(t, float64(100), bi.metrics["rta_implements_calls_test_flavor_4_workers"])

	// TypeCounter
	assert.Equal(t, float64(200), bi.metrics["rta_concrete_types_test_flavor_4_workers"])
	assert.Equal(t, float64(50), bi.metrics["rta_interface_types_test_flavor_4_workers"])

	// ImplementsSuccessFailCounter
	assert.Equal(t, float64(80), bi.metrics["rta_implements_success_test_flavor_4_workers"])
	assert.Equal(t, float64(20), bi.metrics["rta_implements_fail_test_flavor_4_workers"])

	// ImplementsSourceCounter
	assert.Equal(t, float64(60), bi.metrics["rta_checks_from_interfaces_test_flavor_4_workers"])
	assert.Equal(t, float64(40), bi.metrics["rta_checks_from_implementations_test_flavor_4_workers"])

	// ImplementsDirectionCounter
	assert.Equal(t, float64(5), bi.metrics["rta_fails_from_interfaces_test_flavor_4_workers"])
	assert.Equal(t, float64(55), bi.metrics["rta_success_from_interfaces_test_flavor_4_workers"])
	assert.Equal(t, float64(15), bi.metrics["rta_fails_from_implementations_test_flavor_4_workers"])
	assert.Equal(t, float64(25), bi.metrics["rta_success_from_implementations_test_flavor_4_workers"])

	// CallSiteCounter
	assert.Equal(t, float64(500), bi.metrics["rta_static_call_sites_test_flavor_4_workers"])
	assert.Equal(t, float64(30), bi.metrics["rta_indirect_call_sites_test_flavor_4_workers"])
	assert.Equal(t, float64(70), bi.metrics["rta_invoke_call_sites_test_flavor_4_workers"])
	assert.Equal(t, float64(600), bi.metrics["rta_total_call_sites_test_flavor_4_workers"])

	// DedupTiming
	assert.Equal(t, 10.5, bi.metrics["rta_pre_dedup_duration_ms_test_flavor_4_workers"])
	assert.Equal(t, 12.0, bi.metrics["rta_post_dedup_duration_ms_test_flavor_4_workers"])
	assert.Equal(t, 100.0, bi.metrics["rta_pre_dedup_heap_delta_mb_test_flavor_4_workers"])
	assert.Equal(t, 95.0, bi.metrics["rta_post_dedup_heap_delta_mb_test_flavor_4_workers"])
	assert.Equal(t, float64(42), bi.metrics["rta_duplicated_edges_test_flavor_4_workers"])

	// IndexStats
	assert.Equal(t, float64(1), bi.metrics["rta_mc_bucket_p50_test_flavor_4_workers"])
	assert.Equal(t, float64(50), bi.metrics["rta_mc_bucket_p100_test_flavor_4_workers"])
	assert.Equal(t, float64(2), bi.metrics["rta_mi_bucket_p50_test_flavor_4_workers"])
	assert.Equal(t, float64(10), bi.metrics["rta_mi_bucket_p100_test_flavor_4_workers"])
	assert.Equal(t, 0.1, bi.metrics["rta_frac_iface_methods_le1_test_flavor_4_workers"])

	// MethodStats
	assert.Equal(t, float64(3), bi.metrics["rta_methods_per_concrete_p50_test_flavor_4_workers"])
	assert.Equal(t, float64(25), bi.metrics["rta_methods_per_concrete_p100_test_flavor_4_workers"])
	assert.Equal(t, float64(1), bi.metrics["rta_methods_per_interface_p50_test_flavor_4_workers"])
	assert.Equal(t, 0.4, bi.metrics["rta_frac_concrete_methods_le1_test_flavor_4_workers"])
}

// mockRTAResultMinimal only implements the basic result (no optional metric interfaces).
type mockRTAResultMinimal struct{}

func (m *mockRTAResultMinimal) GetCallGraph() *callgraph.Graph { return nil }
func (m *mockRTAResultMinimal) GetReachable() map[*ssa.Function]struct{ AddrTaken bool } {
	return nil
}
func (m *mockRTAResultMinimal) GetRuntimeTypes() typeutil.Map { return typeutil.Map{} }
func (m *mockRTAResultMinimal) Materialize() *rtapkg.Result   { return nil }

func TestCollectRTAMetricsMinimal(t *testing.T) {
	// A result that doesn't implement any optional interfaces should not panic.
	logger := zap.NewNop().Sugar()
	b := NewBuilder(logger, nil, nil, nil, mapper.SSAASTMap{}, mapper.SignatureASTMap{}, false, nil, nil, nil, false, DefaultSerialGraphConfig())
	bi := b.(*builderImpl)

	mock := &mockRTAResultMinimal{}
	bi.collectRTAMetrics(mock, "minimal", 1)

	// No metric keys should be set for interfaces not implemented.
	_, hasImplements := bi.metrics["rta_implements_calls_minimal_1_workers"]
	assert.False(t, hasImplements)
	_, hasTypes := bi.metrics["rta_concrete_types_minimal_1_workers"]
	assert.False(t, hasTypes)
}

func TestComputeNeighborHash(t *testing.T) {
	logger := zap.NewNop().Sugar()
	b := NewBuilder(logger, nil, nil, nil, mapper.SSAASTMap{}, mapper.SignatureASTMap{}, false, nil, nil, nil, false, DefaultSerialGraphConfig())
	bi := b.(*builderImpl)

	// Empty graph — should not panic, should set metric.
	cg := &callgraph.Graph{Nodes: map[*ssa.Function]*callgraph.Node{}}
	bi.computeNeighborHash(cg, "empty", 1)

	_, hasHash := bi.metrics["rta_callgraph_neighbor_hash_empty_1_workers"]
	assert.True(t, hasHash)

	// Graph with nil function key — should be skipped by the nil guard.
	node := &callgraph.Node{}
	cg = &callgraph.Graph{Nodes: map[*ssa.Function]*callgraph.Node{nil: node}}
	bi.computeNeighborHash(cg, "nilkey", 1)

	_, hasHash = bi.metrics["rta_callgraph_neighbor_hash_nilkey_1_workers"]
	assert.True(t, hasHash)
}

// loadInvokeCallsBuilder loads the small invokecalls exampleapp and returns a builderImpl
// suitable for integration testing of RTA sweep and flavor execution.
func loadInvokeCallsBuilder(t *testing.T, cfg Config) *builderImpl {
	t.Helper()
	_, allPkgs, err := loader.LoadWithGoPath(
		"rpcshield",
		[]string{"exampleapps", "invokecalls", "invokecalls-anal"},
		"github.com/uber-research/rpcshield/go/exampleapps/invokecalls",
	)
	require.NoError(t, err)

	prog, allSSAPkgs := ssautil.AllPackages(allPkgs, ssa.InstantiateGenerics|ssa.GlobalDebug)
	prog.Build()

	b := NewBuilder(
		zap.NewNop().Sugar(), allPkgs, prog, allSSAPkgs,
		mapper.SSAASTMap{}, mapper.SignatureASTMap{},
		false, nil, nil, nil, false, cfg,
	)
	return b.(*builderImpl)
}

func TestRunSingleFlavorCustomRTA(t *testing.T) {
	// Exercises the default branch in runSingleFlavor (custom RTA via NewImpl).
	bi := loadInvokeCallsBuilder(t, DefaultSerialGraphConfig())

	entry, err := bi.getRTAEntryFunctions()
	require.NoError(t, err)
	require.NotEmpty(t, entry)

	// Run SRTA (a custom flavor that goes through NewImpl).
	cg, err := bi.runSingleFlavor(context.Background(), entry, FlavorSRTA, 1)
	require.NoError(t, err)
	require.NotNil(t, cg)
	assert.Greater(t, len(cg.Nodes), 0)
}

func TestRunSingleFlavorStdlib(t *testing.T) {
	bi := loadInvokeCallsBuilder(t, DefaultSerialGraphConfig())

	entry, err := bi.getRTAEntryFunctions()
	require.NoError(t, err)

	cg, err := bi.runSingleFlavor(context.Background(), entry, FlavorStdlib, 1)
	require.NoError(t, err)
	require.NotNil(t, cg)
	assert.Greater(t, len(cg.Nodes), 0)
}

func TestRunSingleFlavorCHA(t *testing.T) {
	bi := loadInvokeCallsBuilder(t, DefaultSerialGraphConfig())

	entry, err := bi.getRTAEntryFunctions()
	require.NoError(t, err)

	cg, err := bi.runSingleFlavor(context.Background(), entry, FlavorCHA, 1)
	require.NoError(t, err)
	require.NotNil(t, cg)
	assert.Greater(t, len(cg.Nodes), 0)
}

func TestBuildCGOnlySweepStdlib(t *testing.T) {
	cfg := DefaultSerialGraphConfig()
	cfg.RTABenchFlavors = []RTAFlavor{FlavorStdlib}
	cfg.RTABenchNumWorkers = []int{1}
	bi := loadInvokeCallsBuilder(t, cfg)

	result, err := bi.BuildCGOnly()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Metrics)

	// Should have raw graph stats.
	_, hasNodes := result.Metrics["rawGraphNumNodes"]
	assert.True(t, hasNodes)
}

func TestBuildCGOnlySweepWithIsomorphism(t *testing.T) {
	cfg := DefaultSerialGraphConfig()
	cfg.RTABenchFlavors = []RTAFlavor{FlavorSRTA}
	cfg.RTABenchNumWorkers = []int{1}
	cfg.CheckIsomorphism = true
	bi := loadInvokeCallsBuilder(t, cfg)

	metrics, err := bi.buildCGOnlySweep()
	require.NoError(t, err)
	require.NotEmpty(t, metrics)

	// Isomorphism check should pass (stdlib prepended automatically).
	assert.Equal(t, float64(1), metrics["isomorphism_check_passed"])
	assert.Equal(t, float64(1), metrics["isomorphism_check_srta_1_workers_passed"])
}

func TestBuildCGOnlySweepCustomFlavor(t *testing.T) {
	cfg := DefaultSerialGraphConfig()
	cfg.RTABenchFlavors = []RTAFlavor{FlavorSRTAKumo}
	cfg.RTABenchNumWorkers = []int{1}
	bi := loadInvokeCallsBuilder(t, cfg)

	metrics, err := bi.buildCGOnlySweep()
	require.NoError(t, err)
	require.NotEmpty(t, metrics)
}

func TestCompareCallGraphsWithRealFunctions(t *testing.T) {
	// Use the invokecalls app to get real SSA functions for edge comparison.
	bi := loadInvokeCallsBuilder(t, DefaultSerialGraphConfig())

	entry, err := bi.getRTAEntryFunctions()
	require.NoError(t, err)

	cg1, err := bi.runSingleFlavor(context.Background(), entry, FlavorStdlib, 1)
	require.NoError(t, err)

	cg2, err := bi.runSingleFlavor(context.Background(), entry, FlavorSRTA, 1)
	require.NoError(t, err)

	logger := zap.NewNop().Sugar()
	iso, diff := CompareCallGraphs(cg1, cg2, logger)
	assert.True(t, iso, "stdlib and srta should produce isomorphic graphs: %s", diff)
}

func TestComputeNeighborHashWithRealFunctions(t *testing.T) {
	bi := loadInvokeCallsBuilder(t, DefaultSerialGraphConfig())

	entry, err := bi.getRTAEntryFunctions()
	require.NoError(t, err)

	cg, err := bi.runSingleFlavor(context.Background(), entry, FlavorStdlib, 1)
	require.NoError(t, err)

	bi.computeNeighborHash(cg, "stdlib", 1)

	_, hasHash := bi.metrics["rta_callgraph_neighbor_hash_stdlib_1_workers"]
	assert.True(t, hasHash)
}

// Verify that the mock types satisfy the rta interfaces at compile time.
var _ rta.ImplementsCounter = (*mockRTAResult)(nil)
var _ rta.TypeCounter = (*mockRTAResult)(nil)
var _ rta.ImplementsSuccessFailCounter = (*mockRTAResult)(nil)
var _ rta.ImplementsSourceCounter = (*mockRTAResult)(nil)
var _ rta.ImplementsDirectionCounter = (*mockRTAResult)(nil)
var _ rta.CallSiteCounter = (*mockRTAResult)(nil)
var _ rta.DedupTiming = (*mockRTAResult)(nil)
var _ rta.IndexStats = (*mockRTAResult)(nil)
var _ rta.MethodStats = (*mockRTAResult)(nil)
