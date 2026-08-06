package types

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/grail"
	"golang.org/x/tools/go/packages"
)

func TestMergeOutboundPoints(t *testing.T) {
	t.Parallel()

	point1, point2, point3 := createTestOutboundPoints(t)

	tests := []struct {
		name     string
		a        []OutboundPoint
		b        []OutboundPoint
		expected []OutboundPoint
	}{
		{
			name:     "empty slices",
			a:        []OutboundPoint{},
			b:        []OutboundPoint{},
			expected: []OutboundPoint{},
		},
		{
			name:     "one empty slice",
			a:        []OutboundPoint{point1},
			b:        []OutboundPoint{},
			expected: []OutboundPoint{point1},
		},
		{
			name:     "no duplicates",
			a:        []OutboundPoint{point1},
			b:        []OutboundPoint{point2},
			expected: []OutboundPoint{point1, point2},
		},
		{
			name:     "with duplicates",
			a:        []OutboundPoint{point1, point2},
			b:        []OutboundPoint{point1, point3},
			expected: []OutboundPoint{point1, point2, point3},
		},
		{
			name:     "all duplicates",
			a:        []OutboundPoint{point1},
			b:        []OutboundPoint{point1},
			expected: []OutboundPoint{point1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeOutboundPoints(tt.a, tt.b)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestOutboundPoint_MarshalJSON(t *testing.T) {
	t.Parallel()

	point1, point2, point3 := createTestOutboundPoints(t)

	for _, point := range []OutboundPoint{point1, point2, point3} {
		data, err := json.Marshal(point)
		require.NoError(t, err)
		require.NotEmpty(t, data)
	}
}

func createTestOutboundPoints(t *testing.T) (OutboundPoint, OutboundPoint, OutboundPoint) {
	t.Helper()

	// Create test packages
	pkg1 := &packages.Package{PkgPath: "test/pkg1", Fset: token.NewFileSet()}
	pkg2 := &packages.Package{PkgPath: "test/pkg2", Fset: token.NewFileSet()}

	// Create test call expressions
	call1 := &ast.CallExpr{Fun: &ast.Ident{Name: "testCall1"}}
	call2 := &ast.CallExpr{Fun: &ast.Ident{Name: "testCall2"}}
	call3 := &ast.CallExpr{Fun: &ast.Ident{Name: "testCall3"}}

	// Create test services
	service1 := grail.Service{Name: "service1", Tier: 1, GitURI: "git://test1"}
	service2 := grail.Service{Name: "service2", Tier: 2, GitURI: "git://test2"}

	point1 := OutboundPoint{Pkg: pkg1, Call: call1, Service: service1, CanonicalName: "service1_call1"}
	point2 := OutboundPoint{Pkg: pkg2, Call: call2, Service: service2, CanonicalName: "service2_call2"}
	point3 := OutboundPoint{Pkg: pkg2, Call: call3, Service: service2, CanonicalName: "service2_call3"}

	return point1, point2, point3
}
