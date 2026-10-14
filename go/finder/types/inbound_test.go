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
