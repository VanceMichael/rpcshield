package asthelper

import (
	"go/ast"
	"go/types"
	"regexp"
)

// IsModeledFuncForErrNonnil checks a function call AST node to see if it is one of the trusted functions, and if it is
// then return true, otherwise false.
func (a *ASTContext) IsModeledFuncForErrNonnil(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	if a.matchTrustedFuncs(call) {
		return true
	}

	// Check if the function is an error wrapper function
	return a.ErrorWrapperFunc(call) != nil
}

func (a *ASTContext) matchTrustedFuncs(call *ast.CallExpr) bool {
	for _, f := range trustedFuncs {
		if f.match(call, a) {
			return true
		}
	}
	return false
}

var _newErrorFuncNameRegex = regexp.MustCompile(`(?i)new[^ ]*error[^ ]*`)

// ErrorWrapperFunc implements a heuristic to identify error wrapper functions (e.g., `errors.Wrapf(err, "message")`).
// It does this by applying the following criteria:
// - The function must have at least one argument of error-implementing type.
// - The function can return several values, but at least one of them must be of error-implementing type.
// It also checks if the function creates new errors, instead of wrapping existing ones (e.g., `NewInternalError("string msg")`).
//
// If the call expression is an error wrapper function, it returns the raw error argument expression, nil otherwise.
func (a *ASTContext) ErrorWrapperFunc(expr *ast.CallExpr) ast.Expr {
	funcObj := a.FuncObjectFromCallExpr(expr)

	// Return early if the function object is nil or does not return an error.
	if funcObj == nil || !FuncIsErrReturning(funcObj) {
		return nil
	}

	// Check if the function is an error wrapper: consumes an error and returns an error.
	for _, arg := range expr.Args {
		// Check if the argument is a call expression.
		if callExpr, ok := arg.(*ast.CallExpr); ok {
			if a.matchTrustedFuncs(callExpr) {
				// Check if the argument is a trusted error returning function call.
				// Example: `wrapError(errors.New("new error"))`
				return arg
			}

			// This is to cover the case `NewInternalError(err.Error())` where the argument is a method call on an error.
			// We want to extract the raw error argument `err` in this case.
			if s, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if a.IsError(s.X) {
					return s.X
				}
			}

			// Recursively check if the argument is an error wrapper function call.
			// Example: `wrapError(wrapError(wrapError(err)))`
			if wrappedArg := a.ErrorWrapperFunc(callExpr); wrappedArg != nil {
				return wrappedArg
			}
		}

		if a.IsError(arg) {
			// Return the raw error argument expression
			return arg
		}
	}

	// Check if the function is creating a new error:
	// - consumes a message string and returns an error
	// - matches regex "new*error" as its function name (e.g., `NewInternalError()`)
	if _newErrorFuncNameRegex.MatchString(funcObj.Name()) {
		for i := 0; i < funcObj.Signature().Params().Len(); i++ {
			param := funcObj.Signature().Params().At(i)
			if t, ok := param.Type().(*types.Basic); ok && t.Kind() == types.String && i < len(expr.Args) {
				return expr.Args[i]
			}
		}
	}

	return nil
}

// trustedExprSig defines the signature of an expression that we "trust".
// For example, we trust `errors.New` and `fmt.Errorf` functions to return non-nil errors.
type trustedExprSig struct {
	enclosingPathRegex *regexp.Regexp
	nameRegex          *regexp.Regexp
}

// match checks if a given expression matches with a trusted expression's signature. Namely,
// it performs a strict matching for the expression name (e.g., function/method or variable) and a user-defined regex
// match for the enclosing package or struct path.
func (t *trustedExprSig) match(expr ast.Expr, a *ASTContext) bool {
	// Get identifier from the expression
	var exprIdent *ast.Ident
	switch expr := expr.(type) {
	case *ast.CallExpr:
		exprIdent = a.FuncIdentFromCallExpr(expr)
	case *ast.SelectorExpr:
		exprIdent = expr.Sel
	case *ast.Ident:
		exprIdent = expr
	}

	// Return early if the expression does not contain an identifier
	if exprIdent == nil {
		return false
	}
	// Match the identifier with the expected name specified in `t`
	if !t.nameRegex.MatchString(exprIdent.Name) {
		return false
	}

	// Return early if the enclosing package or struct path is not specified
	if t.enclosingPathRegex == nil {
		return true
	}

	// Get the types.Object of the expression to match the enclosing package or struct path.
	obj := a.ObjectOf(exprIdent)
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	pkgPath := obj.Pkg().Path()

	// Add struct name to the path if the expression is a method: "<pkg path>.<struct name>".
	// E.g., for `e.Unwrap(err).Error(err)`, path = github.com/pkg/errors
	if funcObj, ok := obj.(*types.Func); ok {
		if recv := funcObj.Type().(*types.Signature).Recv(); recv != nil {
			if n, ok := UnwrapPtr(recv.Type()).(*types.Named); ok {
				pkgPath = pkgPath + "." + n.Obj().Name()
			}
		}
	}

	return t.enclosingPathRegex.MatchString(pkgPath)
}

// trustedFuncs defines the map of trusted functions and their actions
var trustedFuncs = []trustedExprSig{
	// standard library errors package. E.g., `errors.New`
	{
		enclosingPathRegex: regexp.MustCompile(`^errors$`),
		nameRegex:          regexp.MustCompile(`^New$|^Join$|^Unwrap$`),
	},

	// standard library fmt package. E.g.,`fmt.Errorf`
	{
		enclosingPathRegex: regexp.MustCompile(`^fmt$`),
		nameRegex:          regexp.MustCompile(`^Errorf$`),
	},

	// `github.com/pkg/errors`
	{
		enclosingPathRegex: regexp.MustCompile(`github\.com/pkg/errors$`),
		nameRegex:          regexp.MustCompile(`^Errorf$|^New$|^Unwrap$|^WithMessage$|^WithMessagef$|^WithStack$|^Wrap$|^Wrapf$`),
	},
}
