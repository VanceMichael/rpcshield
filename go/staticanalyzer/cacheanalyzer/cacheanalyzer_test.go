package cacheanalyzer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
)

func TestAnalyzeCallNotFound(t *testing.T) {
	// Create a new cache analyzer.
	ca := NewCacheAnalyzer()

	// Create dummy function and call instruction.
	dummyFunc := &ssa.Function{}
	dummyCall := &ssa.Call{}

	// Since nothing is inserted, AnalyzeCall should return FailUnknown with an error.
	fail, err := ca.AnalyzeCall(dummyFunc, dummyCall)
	require.Error(t, err)
	require.Equal(t, constant.FailUnknown, fail.Status)
}

func TestInsertAndAnalyzeCall(t *testing.T) {
	// Create a new cache analyzer.
	ca := NewCacheAnalyzer()

	// Create dummy function and call instruction.
	dummyFunc := &ssa.Function{}
	dummyCall := &ssa.Call{}

	// Insert a specific failness (using FailSome as a non-default value).
	expectedFail := constant.FailClose

	ca.InsertToCache(dummyFunc, dummyCall, expectedFail, constant.SSA)

	// Now AnalyzeCall should return the inserted failness without error.
	fail, err := ca.AnalyzeCall(dummyFunc, dummyCall)
	require.NoError(t, err)
	require.Equal(t, expectedFail, fail.Status)
	require.Equal(t, constant.SSA, fail.Source)
}
