package function

import (
	"fmt"
	"go/ast"
	"go/types"
	"runtime/debug"

	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/output"
	"github.com/uber-research/rpcshield/go/staticanalyzer/asthelper"
	"go.uber.org/zap"
)

type callerCalleeInfo struct {
	caller *ast.FuncDecl // Can be a named or anonymous function (*ast.FuncLit) wrapped in a fake FuncDecl
	callee *ast.CallExpr

	// The following field is set during the analysis (inside `collectCallerCalleePairs` function)
	callerObj *types.Func
}

// funcAnalyzer is a struct that contains the necessary information to analyze a function for error propagation.
type funcAnalyzer struct {
	*callerCalleeInfo
	astContext *asthelper.ASTContext
	logger     *zap.SugaredLogger
}

// Run statically analyzes a function for error propagation. It performs the following steps:
// (1) Check if the function is a valid candidate for error propagation analysis
// (2) Perform data flow analysis to track error propagation
// (3) Determine the status of the function based on the analysis results
func Run(caller *ast.FuncDecl, callee *ast.CallExpr, astContext *asthelper.ASTContext, logger *zap.SugaredLogger) (analyzerOutput *output.AnalyzerOutput, err error) {
	// As a last resort, we recover from a panic when running the analysis, convert the panic to an error and return.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("INTERNAL PANIC IN STATIC ANALYZER: %v: \n%s", r, string(debug.Stack()))
		}
	}()

	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	var callerCalleePairs []*callerCalleeInfo
	collectCallerCalleePairs(caller, callee, astContext, &callerCalleePairs)

	if len(callerCalleePairs) == 0 {
		s, formatErr := asthelper.FormatExpr(callee, astContext.PkgInfo.Pkg.Fset)
		if formatErr != nil {
			s = "<error getting callee name>"
		}
		return nil, fmt.Errorf("no caller-callee pairs found for caller `%s` and callee `%s`", caller.Name.Name, s)
	}

	analyzerOutput = &output.AnalyzerOutput{Status: constant.FailUnknown, Source: constant.AST, OriginalSource: constant.AST}
	for _, pair := range callerCalleePairs {
		if pair.caller == nil || pair.callee == nil || pair.callerObj == nil {
			return nil, fmt.Errorf("caller or callee is nil in the caller-callee pair")
		}

		// Create a new funcAnalyzer object for each caller-callee pair
		f := &funcAnalyzer{
			callerCalleeInfo: pair,
			astContext:       astContext,
			logger:           logger,
		}

		if !asthelper.FuncIsErrReturning(f.callerObj) {
			// If a function is not error returning, then by definition it is fail open.
			analyzerOutput.Status = constant.FailOpen
			analyzerOutput.DiagnosticMsg = constant.FailOpenNoErrReturningFunc
			return analyzerOutput, nil
		}

		// Step 1: Check if the function is a valid candidate for error propagation analysis
		funcValidity := f.isAnalyzable()
		if funcValidity != constant.FuncValid {
			// If a function does not pass the validity check, then by definition it is fail open.
			analyzerOutput.Status = constant.FailOpen
			analyzerOutput.DiagnosticMsg = constant.FailOpenInvalidFunc
			return analyzerOutput, nil
		}

		// Step 2: Perform data flow analysis to track error propagation
		errRetData := f.backpropAcrossFunc()

		// Step 3: Determine the status of the function based on the analysis results.
		f.determineFailness(errRetData, analyzerOutput)

		if analyzerOutput.Status == constant.FailOpen {
			break
		}
	}

	return analyzerOutput, nil
}

// findEnclosingFunction traverses up the AST parent nodes to find the enclosing function
// and returns the function declaration, its object, and whether it's an anonymous function.
func findEnclosingFunction(parents []ast.Node, astContext *asthelper.ASTContext) (*ast.FuncDecl, *types.Func, bool) {
	for i := len(parents) - 1; i >= 0; i-- {
		switch fn := parents[i].(type) {
		case *ast.FuncLit:
			if info, ok := astContext.FuncLitFuncMap[fn]; ok {
				return info.FakeFuncDecl, info.FakeFuncObj, true
			}
		case *ast.FuncDecl:
			return fn, astContext.FuncObjectFromFuncDecl(fn), false
		}
	}
	return nil, nil, false
}

// collectCallerCalleePairs recursively traverses functions, collecting caller-callee pairs. If the callee is enclosed
// within an anonymous function, it recursively analyzes the enclosing function to determine the hidden caller-callee chain.
// Specifically, 2 cases are considered:
// Case 1: Callee is a static call contained within Caller (no anonymous functions):
//
//	func run() {
//	   return OutboundRPC()
//	}
//
// Output:
// caller: run, callee: OutboundRPC
//
// Case 2: Callee is a static call contained within one (or more) anonymous function within Caller
//
//	func run() (*S, error) {
//		call2 := func() (*S, error) {
//			return OutboundRPC()
//		}
//		call1 := func() (*S, error) {
//			return call2()
//		}
//		return call1()
//	}
//
// Output:
// caller: call2, callee: OutboundRPC
// caller: call1, callee: call2,
// caller: run, callee: call1
func collectCallerCalleePairs(caller *ast.FuncDecl, callee *ast.CallExpr, astContext *asthelper.ASTContext, result *[]*callerCalleeInfo) {
	var parents []ast.Node
	var actualCaller *ast.FuncDecl
	var actualCallerObj *types.Func
	var actualCallerCallExpr *ast.CallExpr
	var isCalleeSiteInAnonymousFunc bool

	ast.Inspect(caller, func(n ast.Node) bool {
		// Pop the current node from the parent stack
		if n == nil {
			if len(parents) > 0 {
				parents = parents[:len(parents)-1]
			}
			return true
		}

		// Append the current node to the parent stack
		parents = append(parents, n)

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if the call site is the callee site
		if call == callee {
			actualCaller, actualCallerObj, isCalleeSiteInAnonymousFunc = findEnclosingFunction(parents, astContext)
		}

		// Check if the call site is the actual caller site, meaning that the callee is enclosed within an anonymous function.
		// If yes, then find the actual caller's CallExpr and its call site position.
		if isCalleeSiteInAnonymousFunc {
			if ident := astContext.FuncIdentFromCallExpr(call); ident != nil {
				if obj := astContext.ObjectOf(ident); obj != nil {
					if funcObj, ok := obj.(*types.Func); ok {
						if asthelper.IsFuncMatch(funcObj, actualCallerObj) {
							actualCallerCallExpr = call
						}
					}
				}
			}
		}

		return true
	})

	if actualCaller != nil {
		*result = append(*result, &callerCalleeInfo{
			caller:    actualCaller,
			callee:    callee,
			callerObj: actualCallerObj,
		})

		// Continue analysis up the call chain if we found the callee inside an anonymous function.
		// This handles nested anonymous functions by recursively collecting the chain of callers
		// until we reach the top-level named function.
		if actualCaller != caller {
			collectCallerCalleePairs(caller, actualCallerCallExpr, astContext, result)
		}
	}
}

// determineFailness determines the failness status of the function based on the error return data. It considers the following cases:
// (1) fail-open if:
//   - the function does not return an error influenced by the return of the call site
//   - the function returns a nil error for all paths influenced by the return of the call site
//
// (2) fail-close if:
//   - the function returns at least one error influenced by the return of the call site, then it is fail close.
//
// (3) fail status unknown if:
//   - the function returns an unknown error value influenced by the return of the call site, then it is fail status unknown.
func (f *funcAnalyzer) determineFailness(errRetData []*ErrorReturnMetadata, analyzerOutput *output.AnalyzerOutput) {
	if len(errRetData) == 0 {
		analyzerOutput.Status = constant.FailOpen
		analyzerOutput.DiagnosticMsg = constant.FailOpenCallSiteNoReturn
	} else {
		allReturnNil := true
		for _, errRet := range errRetData {
			if errRet.Value&constant.ErrorNonnil != 0 {
				allReturnNil = false
				analyzerOutput.Status = constant.FailClose
				analyzerOutput.DiagnosticMsg = constant.FailCloseCallSiteReturn
				break
			}
			if errRet.Value&constant.ErrorUndetermined != 0 {
				allReturnNil = false
				analyzerOutput.DiagnosticMsg = constant.FailUnknownCallSiteReturn
			}
			if errRet.Value&constant.ErrorUndeterminedForCallExpr != 0 {
				allReturnNil = false
				analyzerOutput.Status = constant.FailUnknownIntended
				analyzerOutput.DiagnosticMsg = constant.FailUnknownCallSiteReturn
				break
			}
		}
		if allReturnNil {
			analyzerOutput.Status = constant.FailOpen
			analyzerOutput.DiagnosticMsg = constant.FailOpenCallSiteAllNilReturn
		}
	}
}

// isAnalyzable checks if a function is valid for error propagation analysis based on the following conditions:
// (1) The function has at least one return statement, and not all error returns are nil
// (2) The function calls at least one of the callees of interest (e.g., outbound sink sites)
func (f *funcAnalyzer) isAnalyzable() constant.FuncValidity {
	totalReturnStmts := 0
	nilReturnStmts := 0
	foundCallSite := false

	ast.Inspect(f.caller, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.ReturnStmt:
			if len(n.Results) > 0 {
				errRetExpr := n.Results[len(n.Results)-1]
				if _, value := f.astContext.AnalyzeErrorValue(errRetExpr); value == constant.ErrorNil {
					nilReturnStmts++
				}
			}
			totalReturnStmts++

		case *ast.CallExpr:
			if f.callee == n {
				foundCallSite = true
			}
		}
		return true
	})

	if totalReturnStmts == 0 {
		return constant.FuncInvalidOtherReason
	}
	// Check 3: verify if the function calls at least one of the outbound sink sites
	if !foundCallSite {
		return constant.FuncInvalidOutboundNotFound
	}

	// Check 2: verify if the function has at least one return statement that does not return nil
	if nilReturnStmts == totalReturnStmts {
		return constant.FuncInvalidAllRetNilAndOutboundFound
	}

	return constant.FuncValid
}
