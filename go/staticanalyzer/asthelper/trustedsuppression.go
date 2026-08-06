package asthelper

import (
	"go/ast"
	"regexp"
)

// IsModeledForSuppression checks expr to see if it is one of the trusted functions defined for
// fail-close suppression, and if it is then return true, otherwise false.
func (a *ASTContext) IsModeledForSuppression(expr ast.Expr) bool {
	// Check if the expression is a call expression, selector expression, or an identifier.
	switch expr.(type) {
	case *ast.CallExpr, *ast.SelectorExpr, *ast.Ident:
		// continue to check the expression
	default:
		return false
	}

	for _, f := range trustedSuppressionFuncs {
		if f.match(expr, a) {
			return true
		}
	}
	return false
}

// trustedSuppressionFuncs defines the list of trusted expressions that suppress the error propagation, implying a fail-open.
var trustedSuppressionFuncs = []trustedExprSig{
	{
		enclosingPathRegex: nil, // not enforcing package match, since the function name is unique enough
		nameRegex:          regexp.MustCompile(`^IsDependencyFailureError$`),
	},
	{
		enclosingPathRegex: nil,
		nameRegex:          regexp.MustCompile(`^IsInEmergencyAreaFailOpen$`),
	},
}
