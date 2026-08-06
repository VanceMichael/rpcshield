package pkgdeps

import (
	"fmt"

	"golang.org/x/tools/go/packages"
)

// PackageNode represents a node in the DAG for a Go package.
type PackageNode struct {
	Pkg          *packages.Package
	Dependencies map[*packages.Package]*PackageNode // Map of dependency package pointer to node
}

// DependencyGraph represents the full dependency graph (DAG).
type DependencyGraph struct {
	nodes map[*packages.Package]*PackageNode
}

// NewDependencyGraph initializes a new DependencyGraph.
func NewDependencyGraph(pkgs []*packages.Package) (*DependencyGraph, error) {
	dg := &DependencyGraph{nodes: make(map[*packages.Package]*PackageNode)}
	// Populate the dependency graph
	for _, pkg := range pkgs {
		dg.getOrCreateNode(pkg)
		for _, dep := range pkg.Imports {
			dg.addDependency(pkg, dep)
		}
	}
	if dg.hasCycle() {
		return nil, fmt.Errorf("cycle detected in the dependency graph")
	}
	return dg, nil
}

// addDependency adds a directed edge from package 'from' to package 'to'.
func (dg *DependencyGraph) addDependency(from, to *packages.Package) {
	fromNode := dg.getOrCreateNode(from)
	toNode := dg.getOrCreateNode(to)
	fromNode.Dependencies[to] = toNode
}

// getOrCreateNode fetches an existing node or creates a new one for a package.
func (dg *DependencyGraph) getOrCreateNode(pkg *packages.Package) *PackageNode {
	if node, exists := dg.nodes[pkg]; exists {
		return node
	}
	node := &PackageNode{Pkg: pkg, Dependencies: make(map[*packages.Package]*PackageNode)}
	dg.nodes[pkg] = node
	return node
}

// genericDFS performs a depth-first search from the given package, collecting nodes along the way.
// Ensures no duplicates are added to the result by using the visited map.
func (dg *DependencyGraph) genericDFS(start *packages.Package, collectFn func(*packages.Package) bool) []*packages.Package {
	visited := make(map[*packages.Package]bool)
	var result []*packages.Package

	var dfs func(pkg *packages.Package)
	dfs = func(pkg *packages.Package) {
		if visited[pkg] {
			return
		}
		visited[pkg] = true
		if collectFn(pkg) {
			result = append(result, pkg)
		}
		for dep := range dg.nodes[pkg].Dependencies {
			dfs(dep)
		}
	}
	dfs(start)
	return result
}

// Dependencies returns all unique transitive dependencies of the given package.
func (dg *DependencyGraph) Dependencies(pkg *packages.Package) ([]*packages.Package, error) {
	if _, exists := dg.nodes[pkg]; !exists {
		return nil, fmt.Errorf("package %s not found", pkg.PkgPath)
	}
	deps := dg.genericDFS(pkg, func(p *packages.Package) bool {
		return p != pkg // exclude the starting package itself
	})
	return deps, nil
}

// hasCycle checks for any cycles in the graph.
func (dg *DependencyGraph) hasCycle() bool {
	visited := make(map[*packages.Package]bool)
	recStack := make(map[*packages.Package]bool)

	var dfs func(pkg *packages.Package) bool
	dfs = func(pkg *packages.Package) bool {
		if recStack[pkg] {
			return true // Cycle found
		}
		if visited[pkg] {
			return false
		}
		visited[pkg] = true
		recStack[pkg] = true
		for dep := range dg.nodes[pkg].Dependencies {
			if dfs(dep) {
				return true
			}
		}
		recStack[pkg] = false
		return false
	}

	for pkg := range dg.nodes {
		if dfs(pkg) {
			return true
		}
	}
	return false
}

// IsReachable checks if there is a path from 'from' package to 'to' package.
func (dg *DependencyGraph) IsReachable(from, to *packages.Package) (bool, error) {
	if _, exists := dg.nodes[from]; !exists {
		return false, fmt.Errorf("package %s not found", from.PkgPath)
	}
	if _, exists := dg.nodes[to]; !exists {
		return false, fmt.Errorf("package %s not found", to.PkgPath)
	}
	result := dg.genericDFS(from, func(p *packages.Package) bool { return p == to })
	return len(result) > 0, nil
}
