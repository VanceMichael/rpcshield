package mapper

import (
	"go/ast"
	"go/types"
	"log"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// SignatureASTMap maps types.Signature to the corresponding AST node and vice versa.
type SignatureASTMap struct {
	// StoA maps types.Signature to the corresponding AST node.
	StoA map[*types.Signature]ast.Node
	// AtoS maps AST node to the corresponding types.Signature.
	AtoS map[ast.Node]*types.Signature
}

// SSAASTMap maps *ssa.Function to the corresponding AST node and vice versa.
type SSAASTMap struct {
	// StoA maps *ssa.Function to the corresponding AST node.
	StoA map[*ssa.Function]ast.Node
	// AtoS maps AST node to the corresponding *ssa.Function.
	AtoS map[ast.Node]*ssa.Function
}

// SignatureToASTMap maps types.Signature to the corresponding AST nodes and vice versa.
// TODO: make it singleton.
func SignatureToASTMap(pkgs []*packages.Package) SignatureASTMap {
	signatureToAST := make(map[*types.Signature]ast.Node)

	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue // Skip packages without type information
		}

		// Traverse the AST to find function declarations and literals

		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch fn := n.(type) {
				case *ast.FuncDecl:
					// For function declarations, retrieve the signature
					def, ok := pkg.TypesInfo.Defs[fn.Name]
					if !ok {
						// TODO:integrated with tools logger
						log.Printf("\n SignatureToASTMap: pkg = %v : % v, fn.Name = %v, Body = %v \n", pkg.PkgPath, pkg.Name, fn.Name, fn.Body)
						break // switch
					}

					defT := def.Type()
					if defT == nil {
						// TODO:integrated with tools logger
						log.Printf("\n SignatureToASTMap: pkg = %v : % v, fn.Name = %v, Body = %v defT = nil \n", pkg.PkgPath, pkg.Name, fn.Name, fn.Body)
						break // switch
					}

					if sig, ok := defT.(*types.Signature); ok {
						signatureToAST[sig] = fn
					}
				case *ast.FuncLit:
					// For function literals, retrieve the signature
					if sig, ok := pkg.TypesInfo.Types[fn].Type.(*types.Signature); ok {
						signatureToAST[sig] = fn
					}
				}
				return true // Continue traversal
			})
		}

	}

	// Create a reverse map from AST nodes to signatures
	astToSignature := make(map[ast.Node]*types.Signature, len(signatureToAST))
	for sig, node := range signatureToAST {
		astToSignature[node] = sig
	}

	return SignatureASTMap{
		StoA: signatureToAST,
		AtoS: astToSignature,
	}
}

// SSAToASTMap maps *ssa.Function to the corresponding AST nodes and vice versa.
// TODO: make it singleton.
func SSAToASTMap(pkgs []*packages.Package, ssaPkgs []*ssa.Package, prog *ssa.Program) SSAASTMap {
	ssaToAST := make(map[*ssa.Function]ast.Node)

	// addFn records fn and recurses into its nested function literals. Nested
	// literals of a dead method are also invisible to ssautil.AllFunctions, so
	// they must be walked explicitly.
	var addFn func(fn *ssa.Function)
	addFn = func(fn *ssa.Function) {
		if fn == nil {
			return
		}
		if fn.Syntax() != nil {
			ssaToAST[fn] = fn.Syntax()
		}
		for _, anon := range fn.AnonFuncs {
			addFn(anon)
		}
	}

	// Reachability-based enumeration. This covers every package-level function,
	// exported types' methods, functions reachable through the built IR, and
	// generic instantiations. Keeping instantiations in StoA matters for callers
	// that look up a call-graph node (which may be an instantiation) in StoA.
	for fn := range ssautil.AllFunctions(prog) {
		addFn(fn)
	}

	// ssautil.AllFunctions omits methods of a syntactic named type unless the
	// type is exported or is otherwise reached (via built IR or a materialized
	// runtime type). Methods of unexported (and generic) types can therefore be
	// missing even though ssa built them. rpcshield's inbound/outbound finders
	// scan AST with no reachability filter, so an RPC call inside such a method
	// (e.g. a provider only wired up in tests) would have no AST->SSA entry and
	// fail the strict IB/OB mapping check. Add every source-declared method back
	// via prog.FuncValue, which returns the built function regardless of
	// reachability, and recurse into their nested function literals.
	for _, pkg := range pkgs {
		if pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			named, ok := scope.Lookup(name).Type().(*types.Named)
			if !ok {
				continue
			}
			for i := 0; i < named.NumMethods(); i++ {
				addFn(prog.FuncValue(named.Method(i)))
			}
		}
	}

	// Reverse map from AST nodes to SSA functions. A syntax node shared by a
	// generic origin and its instantiations must resolve to the origin (source)
	// function deterministically: the finders map an *ast.FuncDecl/FuncLit and
	// need the declared function whose body carries the call instructions, not
	// an arbitrary instantiation.
	astToSSA := make(map[ast.Node]*ssa.Function, len(ssaToAST))
	for fn, node := range ssaToAST {
		if existing, ok := astToSSA[node]; ok && len(existing.TypeArgs()) == 0 {
			continue // Keep the source (non-instantiated) function.
		}
		astToSSA[node] = fn
	}

	return SSAASTMap{
		StoA: ssaToAST,
		AtoS: astToSSA,
	}
}
