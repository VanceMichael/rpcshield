package graph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuild_CanceledContextReturnsCanceledError verifies the graph builder
// threads a canceled context through the pipeline and returns an error
// wrapping context.Canceled (rather than a partial/empty result), does no
// work after cancellation, and returns promptly (the RTA pool rejects all
// work units when the context is already canceled).
//
// commonSetup uses the rpcshielddemo whole-program fixture (same as TestE2E)
// and leaves the call-tree in its default parallel mode, so the pre-canceled
// context exercises the RTA entry guard ahead of the parallel CCT workers.
func TestBuild_CanceledContextReturnsCanceledError(t *testing.T) {
	b := commonSetup(t, true).(*builderImpl)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b.config.Context = ctx

	paths, buildResult, err := b.Build()
	require.Error(t, err, "Build on a canceled context must return the cancellation error")
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, paths)
	require.Nil(t, buildResult)
}

// TestBuildCGOnly_CanceledContextReturnsCanceledError covers the benchmark
// sweep entrypoint, which has its own pre-sweep cancellation guard.
func TestBuildCGOnly_CanceledContextReturnsCanceledError(t *testing.T) {
	b := commonSetup(t, true).(*builderImpl)
	b.config.RTABenchFlavors = []RTAFlavor{FlavorStdlib}
	b.config.RTABenchNumWorkers = []int{1}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b.config.Context = ctx

	_, err := b.BuildCGOnly()
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

// TestBuilderContextDefaultsToBackground ensures a builder with no Context
// configured behaves exactly as before the cancellation work.
func TestBuilderContextDefaultsToBackground(t *testing.T) {
	b := commonSetup(t, true).(*builderImpl)
	require.NotNil(t, b.context())
	require.NoError(t, b.context().Err())
	require.NoError(t, b.checkCancel())
}
