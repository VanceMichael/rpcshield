package types

import (
	"encoding/json"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/grail"
	"golang.org/x/tools/go/packages"
)

func TestMergeInboundPoints(t *testing.T) {
	t.Parallel()

	point1, point2, point3 := createTestInboundPoints(t)

	tests := []struct {
		name     string
		a        []InboundPoint
		b        []InboundPoint
		expected []InboundPoint
	}{
		{
			name:     "empty slices",
			a:        []InboundPoint{},
			b:        []InboundPoint{},
			expected: []InboundPoint{},
		},
		{
			name:     "one empty slice",
			a:        []InboundPoint{point1},
			b:        []InboundPoint{},
			expected: []InboundPoint{point1},
		},
		{
			name:     "no duplicates",
			a:        []InboundPoint{point1},
			b:        []InboundPoint{point2},
			expected: []InboundPoint{point1, point2},
		},
		{
			name:     "with duplicates",
			a:        []InboundPoint{point1, point2},
			b:        []InboundPoint{point1, point3},
			expected: []InboundPoint{point1, point2, point3},
		},
		{
			name:     "all duplicates",
			a:        []InboundPoint{point1},
			b:        []InboundPoint{point1},
			expected: []InboundPoint{point1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeInboundPoints(tt.a, tt.b)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestInboundPoint_MarshalJSON(t *testing.T) {
	t.Parallel()

	point1, point2, point3 := createTestInboundPoints(t)

	for _, point := range []InboundPoint{point1, point2, point3} {
		data, err := json.Marshal(point)
		require.NoError(t, err)
		require.NotEmpty(t, data)
	}
}

func createTestInboundPoints(t *testing.T) (InboundPoint, InboundPoint, InboundPoint) {
	t.Helper()

	// Create test packages
	pkg1 := &packages.Package{PkgPath: "test/pkg1", Fset: token.NewFileSet()}
	pkg2 := &packages.Package{PkgPath: "test/pkg2", Fset: token.NewFileSet()}

	// Create test functions
	func1 := types.NewFunc(token.NoPos, nil, "TestFunc1", nil)
	func2 := types.NewFunc(token.NoPos, nil, "TestFunc2", nil)
	func3 := types.NewFunc(token.NoPos, nil, "TestFunc3", nil)

	// Create test services
	service1 := grail.Service{Name: "service1", Tier: 1, GitURI: "git://test1"}
	service2 := grail.Service{Name: "service2", Tier: 2, GitURI: "git://test2"}

	point1 := InboundPoint{Pkg: pkg1, Func: func1, Service: service1, CanonicalName: "service1__func1"}
	point2 := InboundPoint{Pkg: pkg2, Func: func2, Service: service2, CanonicalName: "service2__func2"}
	point3 := InboundPoint{Pkg: pkg2, Func: func3, Service: service2, CanonicalName: "service2__func3"}

	return point1, point2, point3
}
