package asthelper

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"github.com/uber-research/rpcshield/go/common/pkgdeps"
	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/staticanalyzer/anonymousfunc"
	"golang.org/x/tools/go/packages"
)

// NilType is the type of the builtin "nil" value.
var NilType = types.Universe.Lookup("nil").Type()

// ErrorType is the type of the builtin "error" interface.
var ErrorType = types.Universe.Lookup("error").Type()

// errorInterface is the underlying type of the builtin "error" interface.
var errorInterface = ErrorType.Underlying().(*types.Interface)

// ASTContext contains the context information for the AST. Specifically, it contains:
// - Package information for the current package and its dependencies
// - Mapping of anonymous functions to their corresponding FuncLitInfo
// - Mapping of identifiers to their corresponding FuncLit nodes
type ASTContext struct {
	PkgInfo                   *pkgdeps.Info // Package information for the current package and its dependencies
	FuncLitFuncMap            map[*ast.FuncLit]*anonymousfunc.FuncLitInfo
	IdentFuncLitMap           map[*ast.Ident]*ast.FuncLit
	GlobalErrorVarFailnessMap map[*types.Var]constant.ErrorValue
}

// New creates a new instance of the ASTContext struct.
func New(pkgInfo *pkgdeps.Info, node ast.Node) *ASTContext {
	// Collect all anonymous functions in the caller function
	funcLitMap, identMap := anonymousfunc.CollectAnonymousFuncs(pkgInfo.Pkg, node)

	a := &ASTContext{
		PkgInfo:                   pkgInfo,
		FuncLitFuncMap:            funcLitMap,
		IdentFuncLitMap:           identMap,
		GlobalErrorVarFailnessMap: make(map[*types.Var]constant.ErrorValue),
	}

	// Collect all global error variables and their nilability status, and update a.GlobalErrorVarFailnessMap
	a.analyzeGlobalErrorVariables()

	return a
}

// IsNil checks if the given identifier is nil.
func (a *ASTContext) IsNil(expr ast.Expr) bool {
	if obj := a.ObjectOf(expr); obj != nil {
		return types.Identical(obj.Type(), NilType)
	}

	// Ideally the above check should be sufficient, but in cases of injected nil literals, it doesn't work.
	// So we add an additional syntactic check here.
	if ident, ok := expr.(*ast.Ident); ok && ident.Name == "nil" {
		return true
	}

	return false
}

// IsError checks if the given expression is of type error.
func (a *ASTContext) IsError(expr ast.Expr) bool {
	t := a.PkgInfo.Pkg.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	// Check if expr represents an implementation of the error interface
	return implementsError(t)
}

// implementsError checks if the given object implements the error interface. It also covers the case of
// interfaces that embed the error interface.
func implementsError(t types.Type) bool {
	if errorInterface == nil {
		return false
	}
	return types.Implements(t, errorInterface)
}

// AnalyzeErrorValue returns ErrorValue for an error expression `err`. If the value is directly determinable from the
// AST, it returns the value, otherwise it returns ErrorUnknown.
func (a *ASTContext) AnalyzeErrorValue(err ast.Expr) (ast.Expr, constant.ErrorValue) {
	// Keyword nil. E.g., return nil
	if a.IsNil(err) {
		return err, constant.ErrorNil
	}

	// Struct object. E.g., return &myErr{}
	if obj := a.ObjectOf(err); obj != nil {
		t := obj.Type()
		if TypeAsDeeply[*types.Struct](t) != nil {
			return err, constant.ErrorNonnil
		}
	}

	switch expr := err.(type) {
	case *ast.CallExpr:
		// If the call expression is a function call, check if it is an error wrapper function.
		if argErrExpr := a.ErrorWrapperFunc(expr); argErrExpr != nil {
			return argErrExpr, constant.ErrorNonnil
		}

		// Modeled error libraries. E.g., return errors.New("error")
		if a.IsModeledFuncForErrNonnil(expr) {
			return err, constant.ErrorNonnil
		}

	case *ast.BinaryExpr:
		// Conditionals. E.g., if err == nil { ... }
		// Note that we only need to check for the condition `err == nil` here since we canonicalize all nil comparisons in
		// the CFG, such as `err != nil` and `nil == err`, to the `err == nil` format in `cfg.BuildAndTransformCFG()`.
		if expr.Op == token.EQL {
			if a.IsError(expr.X) && a.IsNil(expr.Y) {
				return expr.X, constant.ErrorNil
			}
		}
	}

	// Check if the error expression is a global error variable.
	if obj := a.ObjectOf(err); obj != nil {
		if varObj, ok := obj.(*types.Var); ok {
			if errVal, ok := a.GlobalErrorVarFailnessMap[varObj]; ok {
				return err, errVal
			}
		}
	}

	return err, constant.ErrorUndetermined
}

// ObjectOf returns a types.Object of the given AST expression, nil otherwise.
func (a *ASTContext) ObjectOf(expr ast.Expr) types.Object {
	ident := a.IdentOf(expr)
	if ident == nil {
		return nil
	}

	// First, check if the identifier is found in the caller's scope. If not found, check in the dependent packages.
	obj := a.PkgInfo.Pkg.TypesInfo.ObjectOf(ident)
	if obj == nil {
		for _, dep := range a.PkgInfo.Deps {
			obj = dep.TypesInfo.ObjectOf(ident)
			if obj != nil {
				break
			}
		}
	}

	// Check if the identifier is an anonymous function
	if obj == nil {
		if funcLit, ok := a.IdentFuncLitMap[ident]; ok {
			if lit, ok := a.FuncLitFuncMap[funcLit]; ok {
				obj = lit.FakeFuncObj
			}
		}
	}

	return obj
}

// IdentOf returns the identifier of the given AST expression, nil otherwise. For example,
// - For ident `x`, it returns `x`.
// - For call expr `x.y.z.foo()`, it returns `foo`.
// - For selector expr `x.y`, it returns `y`.
// - For unary expr `*x`, it returns `x`.
// - For composite lit `&S{}`, it returns `S`.
func (a *ASTContext) IdentOf(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.CallExpr:
		return a.FuncIdentFromCallExpr(e)
	case *ast.UnaryExpr:
		return a.IdentOf(e.X)
	case *ast.CompositeLit:
		return a.IdentOf(e.Type)
	case *ast.SelectorExpr:
		return e.Sel
	default:
		return nil
	}
}

// FuncObjectFromCallExpr returns the function object from the call expression, nil otherwise.
func (a *ASTContext) FuncObjectFromCallExpr(callExpr *ast.CallExpr) *types.Func {
	funcIdent := a.FuncIdentFromCallExpr(callExpr)
	if funcIdent == nil {
		return nil
	}
	obj := a.ObjectOf(funcIdent)
	if obj == nil {
		// Check if the call expression is a function literal
		return nil
	}
	funcObj, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	return funcObj
}

// FuncObjectFromFuncDecl returns the function object (*types.Func) from the function declaration (*ast.FuncDecl).
func (a *ASTContext) FuncObjectFromFuncDecl(f *ast.FuncDecl) *types.Func {
	obj := a.ObjectOf(f.Name)
	if obj == nil {
		panic(fmt.Sprintf("function object for `%s` not found", f.Name.Name))
	}
	return obj.(*types.Func) // Safe type assertion since `f` is a FuncDecl
}

// FuncIdentFromCallExpr returns the function identified from a call expression. If the function
// is an anonymous function, it will return the fake function declaration created for that.
func (a *ASTContext) FuncIdentFromCallExpr(expr *ast.CallExpr) *ast.Ident {
	var ident *ast.Ident
	var funcLit *ast.FuncLit

	switch fun := expr.Fun.(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		// if ident is nil, check if the expr represents a FuncLit node
		funcLit, _ = expr.Fun.(*ast.FuncLit)
	}

	if funcLit == nil && ident != nil {
		// check if the ident points to a function literal node
		funcLit = getFuncLitFromAssignment(ident)
	}

	if funcLit != nil {
		if info, ok := a.FuncLitFuncMap[funcLit]; ok {
			return info.FakeFuncDecl.Name
		}
		return nil
	}

	return ident
}

func (a *ASTContext) analyzeGlobalErrorVariables() {
	// Build package list
	allPkgs := make([]*packages.Package, 0, 1+len(a.PkgInfo.Deps))
	if a.PkgInfo.Pkg != nil {
		allPkgs = append(allPkgs, a.PkgInfo.Pkg)
	}
	for _, dep := range a.PkgInfo.Deps {
		if dep != nil {
			allPkgs = append(allPkgs, dep)
		}
	}

	// Analyze each package and collect global error variables and their nilability status.
	// The nilability status is determined by analyzing the RHS of the global variable declaration.
	for _, pkg := range allPkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.VAR || len(genDecl.Specs) == 0 {
					continue
				}

				for _, spec := range genDecl.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					// Process each global variable declaration.
					for i, name := range valueSpec.Names {
						if obj := a.ObjectOf(name); obj != nil {
							if varObj, ok := obj.(*types.Var); ok && implementsError(varObj.Type()) {
								// Get nilability status of the global error variable.
								var errVal constant.ErrorValue
								if i < len(valueSpec.Values) {
									_, errVal = a.AnalyzeErrorValue(valueSpec.Values[i])
								} else {
									// If there is no RHS, it implies that the global variable is uninitialized. Hence, we assume it is nil.
									// Note: There is a possibility that the global variable may be
									// assigned to a non-nil value later in code, but currently we don't track that.
									errVal = constant.ErrorNil
								}
								a.GlobalErrorVarFailnessMap[varObj] = errVal
							}
						}
					}
				}
			}
		}
	}
}

// IsFuncMatch checks if two functions are the same. The two functions are also considered a match if one is a concrete
// method and the other is its interface method, or vice versa.
func IsFuncMatch(f1, f2 *types.Func) bool {
	f1Recv := f1.Type().(*types.Signature).Recv()
	f2Recv := f2.Type().(*types.Signature).Recv()

	if f1Recv == nil || f2Recv == nil || types.Identical(f1Recv.Type(), f2Recv.Type()) {
		return f1 == f2
	}

	// Check if the functions are methods of the same interface implementation. If f1 is an interface method and
	// f2 is its struct method, or vice versa, then they are considered the same.
	isInterfaceStructMethodMatch := func(r1, r2 *types.Var, f1, f2 *types.Func) bool {
		if r1Type := TypeAsDeeply[*types.Interface](r1.Type()); r1Type != nil {
			// Check if r2 implements the interface of r1
			if !types.Implements(r2.Type(), r1Type) {
				return false
			}
			// Get the implemented method of f1 in struct of r2
			f1ImplementedMethodObj, _, _ := types.LookupFieldOrMethod(r2.Type(), false, f2.Pkg(), f1.Name())
			// Check if the implemented method is the same as f2
			return f1ImplementedMethodObj != nil && f1ImplementedMethodObj == f2
		}
		return false
	}

	return isInterfaceStructMethodMatch(f1Recv, f2Recv, f1, f2) || isInterfaceStructMethodMatch(f2Recv, f1Recv, f2, f1)
}

// FormatExpr converts AST expression to string
func FormatExpr(n ast.Node, fset *token.FileSet) (string, error) {
	builder := &strings.Builder{}
	err := printer.Fprint(builder, fset, n)
	return builder.String(), err
}

// FuncIsErrReturning encodes the conditions that a function is deemed "error-returning".
// A function is deemed "error-returning" iff it has a single result of type `error`, and that
// result is the last in the list of results.
func FuncIsErrReturning(fdecl *types.Func) bool {
	results := fdecl.Type().(*types.Signature).Results()
	n := results.Len()
	if n == 0 {
		return false
	}

	errRes := results.At(n - 1)
	if !implementsError(errRes.Type()) {
		return false
	}

	for i := 0; i < n-1; i++ {
		if implementsError(results.At(i).Type()) {
			return false
		}
	}
	return true
}

// UnwrapPtr unwraps a pointer type and returns the element type. For all other types it returns
// the type unmodified.
func UnwrapPtr(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// TypeAsDeeply retrieves the underlying type of `t` and returns it as type `T`. If `t` cannot be represented as `T`,
// it returns the zero value of `T`. For example,
// - if `t` is a pointer to a struct, and `T` is a struct, it returns the struct type.
// - if `t` is a pointer to a struct, and `T` is an interface, it returns the zero value of `T`, which is `nil`.
func TypeAsDeeply[T types.Type](t types.Type) T {
	a, ok := UnwrapPtr(t).Underlying().(T)
	if !ok {
		return *new(T) // Zero value of T.
	}
	return a
}

// getFuncLitFromAssignment if the declaration of the ident is an assignment
// statement and Rhs of the assignment is a call expression which represents an
// anonymous function, returns the ident of the fake function declaration created
// for that. Otherwise, return nil.
func getFuncLitFromAssignment(ident *ast.Ident) *ast.FuncLit {
	if ident.Obj == nil || ident.Obj.Decl == nil {
		return nil
	}

	if assign, ok := ident.Obj.Decl.(*ast.AssignStmt); ok {
		if len(assign.Rhs) != 1 {
			return nil
		}

		if rhs, ok := assign.Rhs[0].(*ast.FuncLit); ok {
			return rhs
		}
	}

	return nil
}
