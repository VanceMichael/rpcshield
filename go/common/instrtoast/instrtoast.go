package instrtoast

import (
	"fmt"
	"go/ast"
	"sync"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// Converter maintains the mapping from ssa.Package to packages.Package,
// a reference to the SSA program, and a cache for already computed results.
type Converter struct {
	prog     *ssa.Program
	pkgMap   map[*ssa.Package]*packages.Package
	cache    map[ssa.Instruction]ast.Node
	cacheMu  sync.RWMutex
	pkgMapMu sync.RWMutex
}

// New creates a new Converter from the given SSA program and go/packages.Package slice.
// The packages must be loaded with syntax information (e.g. packages.NeedSyntax).
func New(prog *ssa.Program, pkgs []*packages.Package) *Converter {
	c := &Converter{
		prog:   prog,
		pkgMap: make(map[*ssa.Package]*packages.Package),
		cache:  make(map[ssa.Instruction]ast.Node),
	}
	// Build the mapping from *ssa.Package to *packages.Package.
	for _, pkg := range pkgs {
		if ssaPkg := prog.Package(pkg.Types); ssaPkg != nil {
			c.pkgMap[ssaPkg] = pkg
		}
	}
	return c
}

// NodeForInstruction returns the AST node that encloses the given ssa.Instruction.
// It uses astutil.PathEnclosingInterval on the corresponding *ast.File from the packages.Package.
// The second return value indicates whether the interval exactly matches the node's boundaries.
// If an error occurs, a non-nil error is returned.
func (c *Converter) NodeForInstruction(inst ssa.Instruction) (ast.Node, bool, error) {
	if inst == nil {
		return nil, false, fmt.Errorf("instruction is nil")
	}
	// Check if the result is cached.
	c.cacheMu.RLock()
	if node, ok := c.cache[inst]; ok {
		c.cacheMu.RUnlock()
		// We assume the cached node was found exactly.
		return node, true, nil
	}
	c.cacheMu.RUnlock()

	// Retrieve the parent function and ssa.Package.
	fn := inst.Parent()
	if fn == nil {
		return nil, false, fmt.Errorf("instruction has no parent function")
	}

	ssaPkg := fn.Pkg
	if ssaPkg == nil {
		return nil, false, fmt.Errorf("function has no associated ssa.Package")
	}

	// Look up the corresponding packages.Package.
	c.pkgMapMu.RLock()
	pkg, ok := c.pkgMap[ssaPkg]
	c.pkgMapMu.RUnlock()
	if !ok {
		return nil, false, fmt.Errorf("no packages.Package found for the given ssa.Package")
	}

	// Ensure the packages.Package has syntax information.
	if pkg.Syntax == nil {
		return nil, false, fmt.Errorf("packages.Package has no Syntax information")
	}

	// Get the FileSet from the SSA program.
	fset := c.prog.Fset

	// Locate the token.File containing the instruction.
	tokenFile := fset.File(inst.Pos())
	if tokenFile == nil {
		return nil, false, fmt.Errorf("could not locate token.File for instruction")
	}

	// Find the *ast.File from pkg.Syntax corresponding to tokenFile.
	var file *ast.File
	for _, f := range pkg.Syntax {
		if fset.File(f.Pos()) == tokenFile {
			file = f
			break
		}
	}
	if file == nil {
		return nil, false, fmt.Errorf("ast.File not found for instruction position")
	}

	// Use astutil.PathEnclosingInterval to obtain the AST node.
	path, exact := astutil.PathEnclosingInterval(file, inst.Pos(), inst.Pos()+1)
	if len(path) == 0 {
		return nil, exact, fmt.Errorf("no AST node found for instruction position")
	}
	node := path[0]

	// Cache the computed node.
	c.cacheMu.Lock()
	c.cache[inst] = node
	c.cacheMu.Unlock()

	return node, exact, nil
}

// CallInstrToCallExpr calls NodeForInstruction and asserts the result to *ast.CallExpr.
// If an error occurs, a non-nil error is returned.
func (c *Converter) CallInstrToCallExpr(callInstr ssa.CallInstruction) (*ast.CallExpr, error) {
	node, _, err := c.NodeForInstruction(callInstr)
	if err != nil {
		return nil, err
	}
	callExpr, ok := node.(*ast.CallExpr)
	if !ok {
		// Handle callExpr extraction when the node is an *ast.GoStmt
		// 1) If the node is not an *ast.CallExpr
		// 2) Test node if it is an *ast.GoStmt
		// 3) Extract the Call field, if true
		// 4) Return an error if both fail
		// Example:
		// func AFunction() {
		//  ...
		// 	go b("")
		//  ...
		// }
		// expected: return b() as the callExpr
		gocallExpr, ok := node.(*ast.GoStmt)
		if !ok {
			return nil, fmt.Errorf("node is not an *ast.CallExpr")
		}
		return gocallExpr.Call, nil
	}
	return callExpr, nil
}
