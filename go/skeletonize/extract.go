package skeletonize

import (
	"fmt"
	"io"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"go.uber.org/zap"
)

// ExtractCode do the code slicing for given func decl name and callee name
func ExtractCode(fileReader io.Reader, funcName string, linenum int, calleeName string, sugaredLogger *zap.SugaredLogger) (string, error) {
	sugaredLogger.Info("Extracting code")
	root, data, err := parseFile(fileReader)
	if err != nil {
		return "", fmt.Errorf("parse file: %w", err)
	}

	funcNode := findFunctionDeclaration(root, funcName, data, linenum)
	if funcNode == nil {
		return "", fmt.Errorf("function %s not found", funcName)
	}

	// Collect LHS variables from specified callee expression, it could be nil
	lhsVars := collectLHSVariables(funcNode, data, calleeName)
	if len(lhsVars) == 0 {
		sugaredLogger.Debug("No LHS variables found for callee expression")
	}
	// Reconstruct the code and clean it up
	modifiedCode := programSlice(funcNode, data, calleeName, lhsVars, 0, sugaredLogger)
	interestVars = make(map[string]bool) // reset global variable
	sugaredLogger.Info("Done extracting code")
	return modifiedCode, nil
}

// findFunctionDeclaration finds the function declaration node in the AST.
func findFunctionDeclaration(node *sitter.Node, funcName string, srccode []byte, linenum int) *sitter.Node {
	//TODO: funcName should not have receiver or package name
	funcName = extractCleanFuncName(funcName)
	switch node.Type() {
	case "function_declaration":
		if node.ChildByFieldName("name").Content(srccode) == funcName {
			return node
		}
	case "method_declaration":
		if node.ChildByFieldName("name").Content(srccode) == funcName {
			return node
		}
	default:
		// BFS search for the function declaration
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			result := findFunctionDeclaration(child, funcName, srccode, linenum)
			if result != nil {
				return result
			}
		}
	}
	return nil
}

func extractCleanFuncName(funcName string) string {
	index := strings.LastIndex(funcName, ".")
	return funcName[index+1:]
}

/*
Collect LHS variables from assignments involving the specified callee expression.
If the callee expression is found in multiple assignments, all LHS variables are collected.
Example:

	// foo is the callee expression
	a, b := foo()
	c := foo()
	...
	return {a, b, c}
*/
func collectLHSVariables(node *sitter.Node, sourceCode []byte, calleeFunctionName string) map[string]bool {
	lhsVars := make(map[string]bool)
	if node.Type() == "short_var_declaration" || node.Type() == "assignment_statement" {
		rhsNode := node.ChildByFieldName("right")
		if rhsNode != nil && containsCallExpression(rhsNode, sourceCode, calleeFunctionName) {
			lhsNode := node.ChildByFieldName("left")
			for i := 0; i < int(lhsNode.NamedChildCount()); i++ {
				varName := strings.TrimSpace(lhsNode.NamedChild(i).Content(sourceCode))
				if varName == "_" {
					continue
				} else {
					lhsVars[varName] = true
				}
			}
		}
	}
	// Recursively collect LHS variables from child nodes
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		for k := range collectLHSVariables(child, sourceCode, calleeFunctionName) {
			lhsVars[k] = true
		}
	}
	return lhsVars
}

// Check if a block contains the specified call-expression
func containsCallExpression(node *sitter.Node, sourceCode []byte, calleeFunctionName string) bool {
	if node.Type() == "call_expression" {
		calleeNode := node.ChildByFieldName("function")
		return calleeNode != nil && string(calleeNode.Content(sourceCode)) == calleeFunctionName
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if containsCallExpression(child, sourceCode, calleeFunctionName) {
			return true
		}
	}
	return false
}

func containsBoolLit(node *sitter.Node, sourceCode []byte) bool {
	if node.Type() == "bool_literal" || node.Type() == "true" || node.Type() == "false" {
		return true
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if containsBoolLit(child, sourceCode) {
			return true
		}
	}
	return false
}
