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
