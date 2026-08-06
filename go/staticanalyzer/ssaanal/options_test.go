package ssaanal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultSCCPOptions(t *testing.T) {
	opts := DefaultSCCPOptions()

	require.True(t, opts.MaybeAnalysisMode, "MaybeAnalysisMode should default to true")
	require.False(t, opts.InverseCheck, "InverseCheck should default to false")
	require.False(t, opts.ForwardPathCheck, "ForwardPathCheck should default to false")
	require.Nil(t, opts.OtherSources, "OtherSources should default to nil")
	require.Nil(t, opts.Logger, "Logger should default to nil")
	require.Nil(t, opts.Metrics, "Metrics should default to nil")
}
