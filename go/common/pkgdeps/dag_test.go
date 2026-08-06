package pkgdeps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// helper function to create a synthetic package
func newPackage(name string, imports ...*packages.Package) *packages.Package {
	pkg := &packages.Package{
		PkgPath: name,
		Name:    name,
		Imports: make(map[string]*packages.Package),
	}
	for _, imp := range imports {
		pkg.Imports[imp.PkgPath] = imp
	}
	return pkg
}

func TestNewDependencyGraph_NoCycles(t *testing.T) {
	// Create synthetic packages with no cycles
	pkgA := newPackage("A")
	pkgB := newPackage("B", pkgA)
	pkgC := newPackage("C", pkgA, pkgB)
	pkgD := newPackage("D", pkgC)

	// Initialize the dependency graph
	graph, err := NewDependencyGraph([]*packages.Package{pkgA, pkgB, pkgC, pkgD})
	require.NoError(t, err, "Expected no error for graph creation without cycles")

	// Check transitive dependencies of each package
	depsA, err := graph.Dependencies(pkgA)
	require.NoError(t, err)
	assert.Empty(t, depsA, "Package A should have no dependencies")

	depsB, err := graph.Dependencies(pkgB)
	require.NoError(t, err)
	assert.ElementsMatch(t, []*packages.Package{pkgA}, depsB, "Package B should depend on A")

	depsC, err := graph.Dependencies(pkgC)
	require.NoError(t, err)
	assert.ElementsMatch(t, []*packages.Package{pkgA, pkgB}, depsC, "Package C should depend transitively on A and B")

	depsD, err := graph.Dependencies(pkgD)
	require.NoError(t, err)
	assert.ElementsMatch(t, []*packages.Package{pkgA, pkgB, pkgC}, depsD, "Package D should depend transitively on A, B, and C")
}

func TestNewDependencyGraph_WithCycle(t *testing.T) {
	// Create synthetic packages with a cycle: A -> B -> C -> A
	pkgA := newPackage("A")
	pkgB := newPackage("B", pkgA)
	pkgC := newPackage("C", pkgB)
	pkgA.Imports["C"] = pkgC // Create cycle A -> B -> C -> A

	// Initialize the dependency graph
	graph, err := NewDependencyGraph([]*packages.Package{pkgA, pkgB, pkgC})
	require.Error(t, err, "Expected error for graph creation with a cycle")
	require.Nil(t, graph, "Graph should be nil when a cycle is detected")
}

func TestIsReachable(t *testing.T) {
	// Create synthetic packages
	pkgA := newPackage("A")
	pkgB := newPackage("B", pkgA)
	pkgC := newPackage("C", pkgB)
	pkgD := newPackage("D", pkgC)
	pkgE := newPackage("E")

	// Initialize the dependency graph
	graph, err := NewDependencyGraph([]*packages.Package{pkgA, pkgB, pkgC, pkgD, pkgE})
	require.NoError(t, err)

	// Test reachability
	reachable, err := graph.IsReachable(pkgD, pkgA)
	require.NoError(t, err)
	assert.True(t, reachable, "Package A should be reachable from D")

	reachable, err = graph.IsReachable(pkgA, pkgD)
	require.NoError(t, err)
	assert.False(t, reachable, "Package D should not be reachable from A")

	reachable, err = graph.IsReachable(pkgA, pkgE)
	require.NoError(t, err)
	assert.False(t, reachable, "Package E should not be reachable from A (no dependency)")
}

func TestDependencies_Transitive(t *testing.T) {
	// Create a more complex package structure for transitive dependencies
	pkgA := newPackage("A")
	pkgB := newPackage("B", pkgA)
	pkgC := newPackage("C", pkgB)
	pkgD := newPackage("D", pkgA, pkgC) // D depends on both A and C

	// Initialize the dependency graph
	graph, err := NewDependencyGraph([]*packages.Package{pkgA, pkgB, pkgC, pkgD})
	require.NoError(t, err)

	// Check transitive dependencies
	depsD, err := graph.Dependencies(pkgD)
	require.NoError(t, err)
	assert.ElementsMatch(t, []*packages.Package{pkgA, pkgB, pkgC}, depsD, "Package D should have transitive dependencies A, B, and C")

	// Test errors for dependencies on a non-existent package
	pkgNotExist := newPackage("NotExist")
	deps, err := graph.Dependencies(pkgNotExist)
	require.Error(t, err, "Expected error for non-existent package")
	assert.Nil(t, deps, "Dependencies should be nil for non-existent package")
}
