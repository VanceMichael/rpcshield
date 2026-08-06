package ssaanal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ssa"
)

func TestFlipCheckType(t *testing.T) {
	tests := []struct {
		in   nilCheckType
		want nilCheckType
	}{
		{checkEqNil, checkNeqNil},
		{checkNeqNil, checkEqNil},
		{notNilCheck, notNilCheck},
	}
	for _, tc := range tests {
		require.Equal(t, tc.want, flipCheckType(tc.in))
	}
}

func TestIsErrorBranch(t *testing.T) {
	tests := []struct {
		name      string
		succIndex int
		checkType nilCheckType
		want      bool
	}{
		{"neqNil true branch is error", 0, checkNeqNil, true},
		{"neqNil false branch is not error", 1, checkNeqNil, false},
		{"eqNil true branch is not error", 0, checkEqNil, false},
		{"eqNil false branch is error", 1, checkEqNil, true},
		{"notNilCheck true branch", 0, notNilCheck, false},
		{"notNilCheck false branch", 1, notNilCheck, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isErrorBranch(tc.succIndex, tc.checkType))
		})
	}
}

func TestCloneNilChecks(t *testing.T) {
	t.Run("nil map", func(t *testing.T) {
		require.Nil(t, cloneNilChecks(nil))
	})

	t.Run("empty map", func(t *testing.T) {
		require.Nil(t, cloneNilChecks(map[ssa.Value]nilCheckType{}))
	})

	t.Run("cloned map is independent", func(t *testing.T) {
		// Use a concrete ssa.Const as a stand-in value
		v := &ssa.Const{}
		original := map[ssa.Value]nilCheckType{v: checkNeqNil}
		cloned := cloneNilChecks(original)
		require.Equal(t, original, cloned)

		// Mutate clone, verify original is unchanged
		cloned[v] = checkEqNil
		require.Equal(t, checkNeqNil, original[v])
	})
}

func TestFlipNilChecks(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result := flipNilChecks(nilCheckInfo{})
		require.Nil(t, result.checks)
	})

	t.Run("flips all checks", func(t *testing.T) {
		v := &ssa.Const{}
		info := nilCheckInfo{
			checks:      map[ssa.Value]nilCheckType{v: checkNeqNil},
			conjunctive: true,
		}
		result := flipNilChecks(info)
		require.Equal(t, checkEqNil, result.checks[v])
		require.True(t, result.conjunctive)
	})
}

func TestUnionNilChecks(t *testing.T) {
	v1 := &ssa.Const{}
	v2 := &ssa.Const{}

	t.Run("both empty", func(t *testing.T) {
		require.Nil(t, unionNilChecks(nil, nil))
	})

	t.Run("one empty", func(t *testing.T) {
		m := map[ssa.Value]nilCheckType{v1: checkNeqNil}
		result := unionNilChecks(m, nil)
		require.Equal(t, checkNeqNil, result[v1])
	})

	t.Run("disjoint", func(t *testing.T) {
		m1 := map[ssa.Value]nilCheckType{v1: checkNeqNil}
		m2 := map[ssa.Value]nilCheckType{v2: checkEqNil}
		result := unionNilChecks(m1, m2)
		require.Len(t, result, 2)
	})

	t.Run("contradiction returns nil", func(t *testing.T) {
		m1 := map[ssa.Value]nilCheckType{v1: checkNeqNil}
		m2 := map[ssa.Value]nilCheckType{v1: checkEqNil}
		require.Nil(t, unionNilChecks(m1, m2))
	})
}

func TestIntersectNilChecks(t *testing.T) {
	v1 := &ssa.Const{}

	t.Run("empty input", func(t *testing.T) {
		result := intersectNilChecks(nil)
		require.Nil(t, result.checks)
	})

	t.Run("single input", func(t *testing.T) {
		info := nilCheckInfo{
			checks:      map[ssa.Value]nilCheckType{v1: checkNeqNil},
			conjunctive: true,
		}
		result := intersectNilChecks([]nilCheckInfo{info})
		require.Equal(t, checkNeqNil, result.checks[v1])
		require.True(t, result.conjunctive)
	})

	t.Run("disagreeing conjunctive returns empty", func(t *testing.T) {
		info1 := nilCheckInfo{
			checks:      map[ssa.Value]nilCheckType{v1: checkNeqNil},
			conjunctive: true,
		}
		info2 := nilCheckInfo{
			checks:      map[ssa.Value]nilCheckType{v1: checkNeqNil},
			conjunctive: false,
		}
		result := intersectNilChecks([]nilCheckInfo{info1, info2})
		require.Nil(t, result.checks)
	})
}

func TestNilChecksEqual(t *testing.T) {
	v := &ssa.Const{}

	require.True(t, nilChecksEqual(nil, nil))
	require.False(t, nilChecksEqual(nil, map[ssa.Value]nilCheckType{v: checkNeqNil}))

	m1 := map[ssa.Value]nilCheckType{v: checkNeqNil}
	m2 := map[ssa.Value]nilCheckType{v: checkNeqNil}
	require.True(t, nilChecksEqual(m1, m2))

	m3 := map[ssa.Value]nilCheckType{v: checkEqNil}
	require.False(t, nilChecksEqual(m1, m3))
}
