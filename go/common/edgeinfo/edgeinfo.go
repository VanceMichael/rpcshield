package edgeinfo

import (
	"go/ast"

	"golang.org/x/tools/go/callgraph"
)

// EdgeInfo represents an edge in the call graph.
type EdgeInfo struct {
	// Edge is the callgraph.Edge object.
	Edge *callgraph.Edge
	// CallExpr is the call expression AST node at the callsite
	CallExpr *ast.CallExpr
}
