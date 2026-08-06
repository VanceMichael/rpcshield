package ssaanal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// Nilness Lattice Tests
// =============================================================================

func TestNilness_String(t *testing.T) {
	tests := []struct {
		name     string
		nilness  nilness
		expected string
	}{
		{"Bottom", nlBottom, "⊥"},
		{"Nil", nlNil, "nil"},
		{"NonNil", nlNonNil, "non-nil"},
		{"Top", nlTop, "⊤"},
		{"Unknown", nilness(999), "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.nilness.String())
		})
	}
}

func TestNilness_Join(t *testing.T) {
	tests := []struct {
		name     string
		n1       nilness
		n2       nilness
		expected nilness
	}{
		// Bottom absorbs anything
		{"Bottom + Bottom", nlBottom, nlBottom, nlBottom},
		{"Bottom + Nil", nlBottom, nlNil, nlNil},
		{"Bottom + NonNil", nlBottom, nlNonNil, nlNonNil},
		{"Bottom + Top", nlBottom, nlTop, nlTop},
		{"Nil + Bottom", nlNil, nlBottom, nlNil},
		{"NonNil + Bottom", nlNonNil, nlBottom, nlNonNil},
		{"Top + Bottom", nlTop, nlBottom, nlTop},

		// Same values stay the same
		{"Nil + Nil", nlNil, nlNil, nlNil},
		{"NonNil + NonNil", nlNonNil, nlNonNil, nlNonNil},
		{"Top + Top", nlTop, nlTop, nlTop},

		// Conflicts produce Top
		{"Nil + NonNil", nlNil, nlNonNil, nlTop},
		{"NonNil + Nil", nlNonNil, nlNil, nlTop},
		{"Nil + Top", nlNil, nlTop, nlTop},
		{"NonNil + Top", nlNonNil, nlTop, nlTop},
		{"Top + Nil", nlTop, nlNil, nlTop},
		{"Top + NonNil", nlTop, nlNonNil, nlTop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.n1.join(tt.n2))
		})
	}
}

// =============================================================================
// Boolean Constant Lattice Tests
// =============================================================================

func TestConstBool_String(t *testing.T) {
	tests := []struct {
		name     string
		b        constBool
		expected string
	}{
		{"Bottom", boolBottom, "⊥"},
		{"False", boolFalse, "false"},
		{"True", boolTrue, "true"},
		{"Top", boolTop, "⊤"},
		{"Unknown", constBool(999), "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.b.String())
		})
	}
}

func TestConstBool_Join(t *testing.T) {
	tests := []struct {
		name     string
		b1       constBool
		b2       constBool
		expected constBool
	}{
		// Bottom absorbs anything
		{"Bottom + Bottom", boolBottom, boolBottom, boolBottom},
		{"Bottom + False", boolBottom, boolFalse, boolFalse},
		{"Bottom + True", boolBottom, boolTrue, boolTrue},
		{"Bottom + Top", boolBottom, boolTop, boolTop},
		{"False + Bottom", boolFalse, boolBottom, boolFalse},
		{"True + Bottom", boolTrue, boolBottom, boolTrue},
		{"Top + Bottom", boolTop, boolBottom, boolTop},

		// Same values stay the same
		{"False + False", boolFalse, boolFalse, boolFalse},
		{"True + True", boolTrue, boolTrue, boolTrue},
		{"Top + Top", boolTop, boolTop, boolTop},

		// Conflicts produce Top
		{"False + True", boolFalse, boolTrue, boolTop},
		{"True + False", boolTrue, boolFalse, boolTop},
		{"False + Top", boolFalse, boolTop, boolTop},
		{"True + Top", boolTrue, boolTop, boolTop},
		{"Top + False", boolTop, boolFalse, boolTop},
		{"Top + True", boolTop, boolTrue, boolTop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.b1.join(tt.b2))
		})
	}
}

func TestConstBool_Negate(t *testing.T) {
	tests := []struct {
		name     string
		b        constBool
		expected constBool
	}{
		{"Negate Bottom", boolBottom, boolBottom},
		{"Negate False", boolFalse, boolTrue},
		{"Negate True", boolTrue, boolFalse},
		{"Negate Top", boolTop, boolTop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.b.negate())
		})
	}
}

// =============================================================================
// Synthetic Lattice Tests
// =============================================================================

func TestSynthetic_String(t *testing.T) {
	tests := []struct {
		name     string
		s        synthetic
		expected string
	}{
		{"Bottom", synthBottom, "⊥"},
		{"False", synthFalse, "false"},
		{"True", synthTrue, "true"},
		{"Unknown", synthetic(999), "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.s.String())
		})
	}
}

func TestSynthetic_Join(t *testing.T) {
	tests := []struct {
		name     string
		s1       synthetic
		s2       synthetic
		expected synthetic
	}{
		// Bottom is the identity
		{"Bottom + Bottom", synthBottom, synthBottom, synthBottom},
		{"Bottom + False", synthBottom, synthFalse, synthFalse},
		{"Bottom + True", synthBottom, synthTrue, synthTrue},
		{"False + Bottom", synthFalse, synthBottom, synthFalse},
		{"True + Bottom", synthTrue, synthBottom, synthTrue},

		// Same values stay the same
		{"False + False", synthFalse, synthFalse, synthFalse},
		{"True + True", synthTrue, synthTrue, synthTrue},

		// OR-join: once true, stays true
		{"False + True", synthFalse, synthTrue, synthTrue},
		{"True + False", synthTrue, synthFalse, synthTrue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.s1.join(tt.s2))
		})
	}
}

// =============================================================================
// Unified Lattice Value Interface Tests
// =============================================================================

func TestJoinLatticeVals(t *testing.T) {
	tests := []struct {
		name     string
		a        latticeVal
		b        latticeVal
		expected latticeVal
	}{
		// nil (bottom) cases
		{"nil + nil", nil, nil, nil},
		{"nil + Nil", nil, nlNil, nlNil},
		{"NonNil + nil", nlNonNil, nil, nlNonNil},
		{"nil + boolTrue", nil, boolTrue, boolTrue},
		{"boolFalse + nil", boolFalse, nil, boolFalse},

		// Same-type joins
		{"Nil + NonNil", nlNil, nlNonNil, nlTop},
		{"boolTrue + boolFalse", boolTrue, boolFalse, boolTop},

		// Cross-type joins produce top of the receiver type
		{"nilness + constBool", nlNil, boolTrue, nlTop},
		{"constBool + nilness", boolTrue, nlNil, boolTop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, joinLatticeVals(tt.a, tt.b))
		})
	}
}

func TestNilness_Equal(t *testing.T) {
	require.True(t, nlNil.Equal(nlNil))
	require.False(t, nlNil.Equal(nlNonNil))
	require.False(t, nlBottom.Equal(nlTop))
}

func TestConstBool_Equal(t *testing.T) {
	require.True(t, boolTrue.Equal(boolTrue))
	require.False(t, boolTrue.Equal(boolFalse))
}

func TestSynthetic_Equal(t *testing.T) {
	require.True(t, synthTrue.Equal(synthTrue))
	require.False(t, synthTrue.Equal(synthFalse))
}

func TestNilness_JoinLattice_Nil(t *testing.T) {
	require.Equal(t, nlNil, nlNil.joinLattice(nil))
}

func TestNilness_JoinLattice_CrossType(t *testing.T) {
	require.Equal(t, nlTop, nlNil.joinLattice(boolTrue))
}

func TestConstBool_JoinLattice_CrossType(t *testing.T) {
	require.Equal(t, boolTop, boolFalse.joinLattice(nlNonNil))
}

func TestConstBool_JoinLattice_Nil(t *testing.T) {
	require.Equal(t, boolTrue, boolTrue.joinLattice(nil))
}

func TestLatticeVal_IsLatticeBottom(t *testing.T) {
	tests := []struct {
		name     string
		v        latticeVal
		expected bool
	}{
		{"nlBottom", nlBottom, true},
		{"nlNil", nlNil, false},
		{"nlTop", nlTop, false},
		{"boolBottom", boolBottom, true},
		{"boolFalse", boolFalse, false},
		{"boolTop", boolTop, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.v.isLatticeBottom())
		})
	}
}
