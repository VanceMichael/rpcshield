package ssaanal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ssa"
)

func TestNewValState(t *testing.T) {
	vs := newValState()
	require.Nil(t, vs.lattice, "lattice should start as nil (bottom)")
	require.True(t, vs.dataSources.isEmpty(), "data sources should start empty")
	require.Equal(t, synthBottom, vs.isSynthetic, "synthetic should start as bottom")
	require.Nil(t, vs.nilInfo.checks, "nil checks should start as nil")
}

func TestValStateGetNilness(t *testing.T) {
	vs := newValState()
	require.Equal(t, nlBottom, vs.getNilness(), "nil lattice returns nlBottom")

	vs.lattice = nlNonNil
	require.Equal(t, nlNonNil, vs.getNilness())

	// Non-nilness lattice returns nlBottom
	vs.lattice = boolTrue
	require.Equal(t, nlBottom, vs.getNilness())
}

func TestValStateGetBoolState(t *testing.T) {
	vs := newValState()
	require.Equal(t, boolBottom, vs.getBoolState(), "nil lattice returns boolBottom")

	vs.lattice = boolTrue
	require.Equal(t, boolTrue, vs.getBoolState())

	// Non-bool lattice returns boolBottom
	vs.lattice = nlNil
	require.Equal(t, boolBottom, vs.getBoolState())
}

func TestValStateSnapshotChanged(t *testing.T) {
	vs := newValState()
	snap := vs.snapshot()
	require.False(t, vs.changed(snap), "unchanged state should not report changed")

	// Change lattice
	vs.lattice = nlNonNil
	require.True(t, vs.changed(snap), "lattice change should be detected")

	// Reset and change sources
	vs.lattice = nil
	snap = vs.snapshot()
	vs.dataSources.add(sourceID(&ssa.Const{}))
	require.True(t, vs.changed(snap), "source change should be detected")

	// Reset and change synthetic
	snap = vs.snapshot()
	vs.isSynthetic = synthTrue
	require.True(t, vs.changed(snap), "synthetic change should be detected")
}

func TestNewBlkState(t *testing.T) {
	bs := newBlkState()
	require.False(t, bs.executable, "block should start as not executable")
	require.Nil(t, bs.nilFacts, "nilFacts should be lazily initialized")
}
