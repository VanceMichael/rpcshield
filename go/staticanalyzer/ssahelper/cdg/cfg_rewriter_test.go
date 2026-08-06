package cdg

// cfg_rewriter_test.go — AST rewriter for extracting CFGs from real-world Go code.
//
// WHAT: Transforms an arbitrary Go function's AST so it compiles using only
// primitive types (int, bool, error) while preserving the exact control flow
// structure: if/else, for, range, switch, type-switch, select, &&, ||, !,
// return, break, continue, goto, and defer.
//
// WHY: The CDG and inverse-CDG path enumeration code can exhibit exponential
// blowup on deeply nested or diamond-shaped control flow found in production
// functions. To regression-test this we need real-world CFGs, but those
// functions live in packages with hundreds of transitive dependencies that
// we cannot (and should not) import in a unit test. The AST rewriter solves
// this: we paste the raw function source into testdata/, rewrite away all
// type dependencies, compile to SSA, and get the exact same CFG shape.
//
// WHERE THE EXAMPLE COMES FROM: The primary test case is the Charge function
// from the service-0001 service (example.internal/part1/service-0001). Its CFG has
// 240+ basic blocks with deeply nested conditionals and loops that cause
// exponential path growth in inverse CDG enumeration. See
// testdata/charge_func.go and charge_regression_test.go.
//
// HOW IT WORKS:
//   1. Parse the raw Go function source (doesn't need to compile).
//   2. Rewrite the AST in place:
//      - Remove receiver (method → function).
//      - Replace all parameters with _pN int.
//      - Replace all results with int (error stays error).
//      - Replace all assignments with calls to helper functions (_v, _ve, _vve)
//        that produce the right number of return values.
//      - Replace all conditions with _b() calls, preserving && / || / !.
//      - Replace range, select, type-switch with equivalent control flow.
//      - LHS variables become package-level globals (_gN int, _geN error).
//   3. Emit compilable source: preamble (helper funcs + globals) + rewritten func.
//   4. Type-check and build SSA → returns the *ssa.Function.

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ssa"
)

// cfgPreamblePrefix is the fixed part of the preamble. After rewriting,
// we append "var _g0, _g1, ... _gN int" for all generated globals.
const cfgPreamblePrefix = `package testpkg

var _n int

func _b() bool { _n++; return _n%2 == 0 }
func _v() int  { _n++; return _n }
func _e() error { return nil }
func _ve() (int, error) { _n++; return _n, nil }
func _vve() (int, int, error) { _n++; return _n, _n, nil }
func _s() []int { return nil }

`

// cfgPreamble returns the full preamble including generated global variables.
func cfgPreamble(numGlobals, numErrGlobals int) string {
	if numGlobals == 0 && numErrGlobals == 0 {
		return cfgPreamblePrefix
	}
	var buf strings.Builder
	buf.WriteString(cfgPreamblePrefix)
	buf.WriteString("var (")
	for i := 0; i < numGlobals; i++ {
		fmt.Fprintf(&buf, "\n\t_g%d int", i)
	}
	for i := 0; i < numErrGlobals; i++ {
		fmt.Fprintf(&buf, "\n\t_ge%d error", i)
	}
	buf.WriteString("\n)\n\n")
	return buf.String()
}

// cfgRewriter rewrites a FuncDecl AST to use only primitive types.
type cfgRewriter struct {
	varCounter     int
	errCounter     int  // count of error-typed globals needed
	numResults     int  // number of return values in rewritten signature
	hasErrorResult bool // whether last result is error
}

// rewriteFuncForCFG rewrites a FuncDecl in place to be compilable with
// only primitive types. Preserves all control flow structure.
func (r *cfgRewriter) rewriteFuncForCFG(fd *ast.FuncDecl) {
	// Remove receiver (method → function).
	fd.Recv = nil

	// Rewrite params: all become _pN int.
	r.rewriteParams(fd.Type.Params)

	// Rewrite results: named types → int, error stays error.
	r.rewriteResults(fd.Type.Results)

	// Rewrite body.
	r.rewriteBlock(fd.Body)
}

func (r *cfgRewriter) rewriteParams(fl *ast.FieldList) {
	if fl == nil {
		return
	}
	var newFields []*ast.Field
	idx := 0
	for range fl.List {
		newFields = append(newFields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(fmt.Sprintf("_p%d", idx))},
			Type:  ast.NewIdent("int"),
		})
		idx++
	}
	fl.List = newFields
}

func (r *cfgRewriter) rewriteResults(fl *ast.FieldList) {
	if fl == nil {
		r.numResults = 0
		return
	}
	var newFields []*ast.Field
	for _, f := range fl.List {
		typStr := exprString(f.Type)
		if typStr == "error" {
			newFields = append(newFields, &ast.Field{Type: ast.NewIdent("error")})
			r.hasErrorResult = true
		} else {
			newFields = append(newFields, &ast.Field{Type: ast.NewIdent("int")})
		}
	}
	fl.List = newFields
	r.numResults = len(newFields)
}

func (r *cfgRewriter) rewriteBlock(block *ast.BlockStmt) {
	if block == nil {
		return
	}
	for i, stmt := range block.List {
		block.List[i] = r.rewriteStmt(stmt)
	}
}

func (r *cfgRewriter) rewriteStmt(stmt ast.Stmt) ast.Stmt {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		if s.Init != nil {
			s.Init = r.rewriteStmt(s.Init)
		}
		s.Cond = r.rewriteCond(s.Cond)
		r.rewriteBlock(s.Body)
		if s.Else != nil {
			s.Else = r.rewriteStmt(s.Else)
		}

	case *ast.ForStmt:
		if s.Init != nil {
			s.Init = r.rewriteStmt(s.Init)
		}
		if s.Cond != nil {
			s.Cond = r.rewriteCond(s.Cond)
		}
		if s.Post != nil {
			s.Post = r.rewriteStmt(s.Post)
		}
		r.rewriteBlock(s.Body)

	case *ast.RangeStmt:
		s.Key = ast.NewIdent("_")
		if s.Value != nil {
			s.Value = ast.NewIdent("_")
		}
		s.X = makeCall("_s")
		s.Tok = token.ASSIGN
		r.rewriteBlock(s.Body)

	case *ast.SwitchStmt:
		if s.Init != nil {
			s.Init = r.rewriteStmt(s.Init)
		}
		if s.Tag != nil {
			s.Tag = makeCall("_v")
		}
		r.rewriteBlock(s.Body)

	case *ast.TypeSwitchStmt:
		// Convert to a regular switch on _v().
		sw := &ast.SwitchStmt{
			Tag:  makeCall("_v"),
			Body: s.Body,
		}
		// Rewrite case clauses: replace type lists with value exprs.
		for _, stmt := range sw.Body.List {
			if cc, ok := stmt.(*ast.CaseClause); ok {
				for i := range cc.List {
					cc.List[i] = makeCall("_v")
				}
				r.rewriteStmtList(cc.Body)
			}
		}
		return sw

	case *ast.CaseClause:
		for i := range s.List {
			s.List[i] = makeCall("_v")
		}
		r.rewriteStmtList(s.Body)

	case *ast.ReturnStmt:
		s.Results = r.makeReturnExprs()

	case *ast.AssignStmt:
		r.rewriteAssign(s)

	case *ast.ExprStmt:
		// Replace expression statement (function call, etc.) with _ = _b().
		s.X = makeCall("_b")

	case *ast.GoStmt:
		// Replace go stmt with expression statement.
		return &ast.ExprStmt{X: makeCall("_b")}

	case *ast.DeferStmt:
		// Replace defer call with a dummy.
		s.Call = makeCallExpr("_b")

	case *ast.DeclStmt:
		// Remove variable declarations — all vars are package-level globals.
		return &ast.ExprStmt{X: makeCall("_b")}

	case *ast.IncDecStmt:
		s.X = ast.NewIdent("_n")

	case *ast.SendStmt:
		return &ast.ExprStmt{X: makeCall("_b")}

	case *ast.BranchStmt:
		// keep as-is (break, continue, goto)

	case *ast.LabeledStmt:
		s.Stmt = r.rewriteStmt(s.Stmt)

	case *ast.BlockStmt:
		r.rewriteBlock(s)

	case *ast.SelectStmt:
		// Convert select to switch on _v().
		sw := &ast.SwitchStmt{
			Tag:  makeCall("_v"),
			Body: &ast.BlockStmt{},
		}
		if s.Body != nil {
			for _, stmt := range s.Body.List {
				if cc, ok := stmt.(*ast.CommClause); ok {
					caseClause := &ast.CaseClause{
						Body: cc.Body,
					}
					if cc.Comm != nil {
						caseClause.List = []ast.Expr{makeCall("_v")}
					}
					r.rewriteStmtList(caseClause.Body)
					sw.Body.List = append(sw.Body.List, caseClause)
				}
			}
		}
		return sw

	case nil:
		// nothing

	default:
		// Unknown statement type — replace with a dummy to avoid compile errors.
		return &ast.ExprStmt{X: makeCall("_b")}
	}
	return stmt
}

func (r *cfgRewriter) rewriteStmtList(stmts []ast.Stmt) {
	for i, s := range stmts {
		stmts[i] = r.rewriteStmt(s)
	}
}

// rewriteCond rewrites a condition expression, preserving && / || / !
// structure (which affects control flow) and replacing leaf expressions
// with _b() calls.
func (r *cfgRewriter) rewriteCond(expr ast.Expr) ast.Expr {
	if expr == nil {
		return makeCall("_b")
	}
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		if e.Op == token.LAND || e.Op == token.LOR {
			e.X = r.rewriteCond(e.X)
			e.Y = r.rewriteCond(e.Y)
			return e
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			e.X = r.rewriteCond(e.X)
			return e
		}
	case *ast.ParenExpr:
		e.X = r.rewriteCond(e.X)
		return e
	}
	return makeCall("_b")
}

// rewriteAssign rewrites an assignment statement. LHS variables become
// package-level globals (_g0, _g1, ... for int; _ge0, _ge1, ... for error)
// so that := can become = without needing local declarations.
func (r *cfgRewriter) rewriteAssign(s *ast.AssignStmt) {
	numLHS := len(s.Lhs)

	// For multi-value calls that return error as last value (_ve, _vve),
	// the last LHS needs an error-typed global.
	switch numLHS {
	case 1:
		s.Lhs = []ast.Expr{ast.NewIdent(fmt.Sprintf("_g%d", r.varCounter))}
		r.varCounter++
		s.Rhs = []ast.Expr{makeCall("_v")}
	case 2:
		s.Lhs = []ast.Expr{
			ast.NewIdent(fmt.Sprintf("_g%d", r.varCounter)),
			ast.NewIdent(fmt.Sprintf("_ge%d", r.errCounter)),
		}
		r.varCounter++
		r.errCounter++
		s.Rhs = []ast.Expr{makeCall("_ve")}
	case 3:
		s.Lhs = []ast.Expr{
			ast.NewIdent(fmt.Sprintf("_g%d", r.varCounter)),
			ast.NewIdent(fmt.Sprintf("_g%d", r.varCounter+1)),
			ast.NewIdent(fmt.Sprintf("_ge%d", r.errCounter)),
		}
		r.varCounter += 2
		r.errCounter++
		s.Rhs = []ast.Expr{makeCall("_vve")}
	default:
		for i := range s.Lhs {
			s.Lhs[i] = ast.NewIdent(fmt.Sprintf("_g%d", r.varCounter))
			r.varCounter++
		}
		s.Rhs = make([]ast.Expr, numLHS)
		for i := range s.Rhs {
			s.Rhs[i] = makeCall("_v")
		}
	}
	s.Tok = token.ASSIGN
}

func (r *cfgRewriter) makeReturnExprs() []ast.Expr {
	if r.numResults == 0 {
		return nil
	}
	exprs := make([]ast.Expr, r.numResults)
	for i := range exprs {
		if i == r.numResults-1 && r.hasErrorResult {
			exprs[i] = makeCall("_e")
		} else {
			exprs[i] = makeCall("_v")
		}
	}
	return exprs
}

// Helper functions for building AST nodes.

func makeCall(name string) ast.Expr {
	return &ast.CallExpr{Fun: ast.NewIdent(name)}
}

func makeCallExpr(name string) *ast.CallExpr {
	return &ast.CallExpr{Fun: ast.NewIdent(name)}
}

func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	default:
		return ""
	}
}

// buildCFGFromGoSource reads a Go source file containing a function
// declaration, rewrites its AST to use only primitive types (preserving
// control flow), compiles it to SSA, and returns the SSA function.
// The source file does NOT need to be compilable — only parseable.
func buildCFGFromGoSource(t *testing.T, srcPath string, funcName string) *ssa.Function {
	t.Helper()

	// Read and parse. Prepend package declaration for parsing.
	raw, err := os.ReadFile(srcPath)
	require.NoError(t, err, "reading source file %s", srcPath)

	wrapped := "package p\n\n" + string(raw)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, srcPath, wrapped, parser.SkipObjectResolution)
	require.NoError(t, err, "parsing source file %s", srcPath)

	// Find the target FuncDecl.
	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Name.Name == funcName {
			funcDecl = fd
			break
		}
	}
	require.NotNil(t, funcDecl, "function %q not found in %s", funcName, srcPath)

	// Rewrite AST.
	rw := &cfgRewriter{}
	rw.rewriteFuncForCFG(funcDecl)

	// Generate compilable source: preamble (with globals) + rewritten function.
	var buf strings.Builder
	buf.WriteString(cfgPreamble(rw.varCounter, rw.errCounter))
	fset2 := token.NewFileSet()
	err = format.Node(&buf, fset2, funcDecl)
	require.NoError(t, err, "formatting rewritten function")

	compilableSrc := buf.String()
	t.Logf("Rewritten source: %d bytes", len(compilableSrc))

	// Parse the compilable source.
	fset3 := token.NewFileSet()
	file2, err := parser.ParseFile(fset3, "rewritten.go", compilableSrc, 0)
	require.NoError(t, err, "parsing rewritten source")

	// Type-check.
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}
	conf := types.Config{}
	pkg, err := conf.Check("testpkg", fset3, []*ast.File{file2}, info)
	require.NoError(t, err, "type-checking rewritten source:\n%s", compilableSrc)

	// Build SSA.
	prog := ssa.NewProgram(fset3, ssa.BuilderMode(0))
	ssaPkg := prog.CreatePackage(pkg, []*ast.File{file2}, info, true)
	ssaPkg.Build()

	fn := ssaPkg.Func(funcName)
	require.NotNil(t, fn, "SSA function %q not found", funcName)
	return fn
}
