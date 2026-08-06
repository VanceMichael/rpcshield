package skeletonize

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"go.uber.org/zap"
)

// init global variable for storing interest variables
var interestVars = make(map[string]bool)

func rewriteDeclSignature(funcNameNode, parametersNode, resultTypeNode, receiverNode *sitter.Node, sourceCode []byte, indentLevel int) string {
	var result strings.Builder
	indent := createIndent(indentLevel)
	result.WriteString(indent)
	result.WriteString(" func ")
	if receiverNode != nil {
		result.WriteString(receiverNode.Content(sourceCode) + " ")
	}
	if funcNameNode != nil {
		result.WriteString(funcNameNode.Content(sourceCode))
	}
	if parametersNode != nil {
		result.WriteString(parametersNode.Content(sourceCode))
	}
	if resultTypeNode != nil {
		result.WriteString(" " + string(resultTypeNode.Content(sourceCode)))
	}
	return result.String()
}

func rewriteDeclBody(bodyNode *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) string {
	var result strings.Builder
	if bodyNode != nil {
		bodyCode := programSlice(bodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel+1, sugaredLogger)
		result.WriteString(bodyCode)
	}
	return result.String()
}

func rewriteExpression(node *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger, sign string) string {
	var result strings.Builder
	indent := createIndent(indentLevel)
	lhsNode := node.ChildByFieldName("left")
	rhsNode := node.ChildByFieldName("right")
	if rhsNode == nil {
		return result.String()
	}

	if containsCallExpression(rhsNode, sourceCode, calleeFunctionName) {
		lhsContent := lhsNode.Content(sourceCode)
		rhsContent := programSlice(rhsNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		if rhsContent != "" {
			result.WriteString(fmt.Sprintf("%s%s %s%s\n", indent, lhsContent, sign, rhsContent))
			// capture transitive variables, but it is conservative
			/*
				a, b = res, t
				if res is an interesting variable. Both LHS vars will be added to interestVars {a, b}
			*/
			for i := 0; i < int(lhsNode.NamedChildCount()); i++ {
				varName := strings.TrimSpace(lhsNode.NamedChild(i).Content(sourceCode))
				if varName == "_" {
					continue
				} else {
					interestVars[varName] = true
				}
			}
		}
	} else if containsInterestingVariables(rhsNode, sourceCode, interestVars) {
		lhsContent := lhsNode.Content(sourceCode)
		rhsContent := programSlice(rhsNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		if rhsContent != "" {
			result.WriteString(fmt.Sprintf("%s%s %s%s\n", indent, lhsContent, sign, rhsContent))
		}
	} else if containsInterestingVariables(lhsNode, sourceCode, interestVars) {
		lhsContent := lhsNode.Content(sourceCode)
		rhsContent := rhsNode.Content(sourceCode)
		result.WriteString(fmt.Sprintf("%s%s %s%s\n", indent, lhsContent, sign, rhsContent))
	}
	return result.String()
}

func rewriteIfBranch(bodyNode *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) string {
	var result strings.Builder
	if bodyNode != nil {
		bodyCode := programSlice(bodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel+1, sugaredLogger)
		result.WriteString(bodyCode)
	}
	return result.String()
}

func rewriteElseBranch(elseNode *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) (string, int) {
	var result strings.Builder
	var elseType int // 0: else if, 1: else
	if elseNode != nil {
		if elseNode.Type() == "if_statement" {
			modiCode := programSlice(elseNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
			result.WriteString(modiCode)
			elseType = 0
		} else if elseNode.Type() == "block" {
			elseCode := programSlice(elseNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
			result.WriteString(elseCode)
			elseType = 1
		} else {
			sugaredLogger.Info("unsupported else type: ", elseNode.Type())

		}
	} else {
		result.WriteString("")
	}
	return result.String(), elseType
}

func rewriteCondition(conditionNode *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) string {
	var result strings.Builder
	if containsInterestingVariables(conditionNode, sourceCode, interestVars) || containsCallExpression(conditionNode, sourceCode, calleeFunctionName) || containsBoolLit(conditionNode, sourceCode) {
		conditionCode := programSlice(conditionNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		if conditionCode != "" {
			result.WriteString(conditionCode)
		}
	}
	return result.String()
}

func branchBody(rewriteIfBranchCode, rewriteElseBranchCode string, elseType int, indent string) string {
	var result strings.Builder

	if rewriteIfBranchCode == "" && rewriteElseBranchCode == "" {
		result.WriteString("\n" + indent + "}\n")
	} else if rewriteIfBranchCode != "" && rewriteElseBranchCode == "" {
		result.WriteString(rewriteIfBranchCode)
		result.WriteString("\n" + indent + "}\n")
	} else if rewriteIfBranchCode == "" && rewriteElseBranchCode != "" {
		result.WriteString("\n" + indent + "}")
		if elseType == 0 {
			result.WriteString(" else")
			result.WriteString(rewriteElseBranchCode) // else if
		} else {
			result.WriteString(" else {\n")
			result.WriteString(rewriteElseBranchCode)
			result.WriteString("\n" + indent + "}\n") // pure else
		}
	} else {
		result.WriteString(rewriteIfBranchCode)
		result.WriteString("\n" + indent + "}")
		if elseType == 0 {
			result.WriteString(" else ")
			result.WriteString(rewriteElseBranchCode)
		} else {
			result.WriteString(" else {\n")
			result.WriteString(rewriteElseBranchCode)
			result.WriteString("\n" + indent + "}\n")
		}
	}
	return result.String()
}

func rewriteBranch(initNode, conditionNode, bodyNode, elseNode *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) string {
	var result strings.Builder
	var initCode string
	indent := createIndent(indentLevel)
	if initNode != nil {
		initCode = programSlice(initNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
	}
	if conditionNode != nil {
		rewriteConditionCode := rewriteCondition(conditionNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		rewriteIfBranchCode := rewriteIfBranch(bodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel+1, sugaredLogger)
		rewriteElseBranchCode, elseType := rewriteElseBranch(elseNode, sourceCode, calleeFunctionName, lhsVars, indentLevel+1, sugaredLogger)

		if initCode != "" && rewriteConditionCode != "" {
			initCode = strings.TrimRight(initCode, "\n")
			result.WriteString(indent + "if " + initCode + ";" + rewriteConditionCode + " {\n")
			result.WriteString(branchBody(rewriteIfBranchCode, rewriteElseBranchCode, elseType, indent))
		} else if initCode == "" && rewriteConditionCode != "" {
			result.WriteString(indent + "if " + rewriteConditionCode + " {\n")
			result.WriteString(branchBody(rewriteIfBranchCode, rewriteElseBranchCode, elseType, indent))
		} else if initCode != "" && rewriteConditionCode == "" {
			result.WriteString(indent + "if " + initCode + " {\n")
			result.WriteString(branchBody(rewriteIfBranchCode, rewriteElseBranchCode, elseType, indent))
		} else {
			if !(rewriteIfBranchCode == "" && rewriteElseBranchCode == "") {
				result.WriteString(indent + "if NOT_CARE {\n")
				result.WriteString(branchBody(rewriteIfBranchCode, rewriteElseBranchCode, elseType, indent))
			}
		}
	}
	return result.String()
}

func rewriteForLoop(node, bodyNode *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) string {
	var result strings.Builder
	if (containsInterestingVariables(node, sourceCode, lhsVars)) || containsCallExpression(node, sourceCode, calleeFunctionName) {
		if bodyNode != nil {
			bodyCode := programSlice(bodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel+1, sugaredLogger)
			result.WriteString(bodyCode)
		}
	}
	return result.String()
}

func rewriteReturn(expressionListNode *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) string {
	var result strings.Builder
	if expressionListNode == nil {
		sugaredLogger.Info("Return statement does not contain an expression list.")
		return " "
	}
	expListCode := programSlice(expressionListNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
	result.WriteString(expListCode)
	return result.String()
}

func rewriteSwitch(node *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) []switchCasePair {
	switchCasePairs := make([]switchCasePair, 0)

	for i := 1; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)

		if int(child.NamedChildCount()) >= 2 {
			caseLabelNode := child.NamedChild(0)
			caseBodyNode := child.NamedChild(1)
			caseLabelCode := programSlice(caseLabelNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
			caseBodyCode := programSlice(caseBodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)

			tempPair := switchCasePair{"case " + caseLabelCode + ": \n", caseBodyCode}
			switchCasePairs = append(switchCasePairs, tempPair)
		} else {
			// defalut case
			caseBodyNode := child.NamedChild(0)
			caseBodyCode := programSlice(caseBodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
			tempPair := switchCasePair{"default :" + "\n", caseBodyCode}
			switchCasePairs = append(switchCasePairs, tempPair)
		}
	}
	return switchCasePairs
}

// Reconstruct the code in recursive way and mark if it hit the callee function.
func programSlice(node *sitter.Node, sourceCode []byte, calleeFunctionName string, lhsVars map[string]bool, indentLevel int, sugaredLogger *zap.SugaredLogger) string {
	if interestVars == nil {
		interestVars = make(map[string]bool)
	}

	var result strings.Builder
	indent := createIndent(indentLevel)
	switch node.Type() {
	case "function_declaration":
		funcNameNode := node.ChildByFieldName("name")
		parametersNode := node.ChildByFieldName("parameters")
		resultTypeNode := node.ChildByFieldName("result")
		bodyNode := node.ChildByFieldName("body")
		result.WriteString(rewriteDeclSignature(funcNameNode, parametersNode, resultTypeNode, nil, sourceCode, indentLevel))
		bodyCode := rewriteDeclBody(bodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		if bodyCode != "" {
			result.WriteString("{\n")
			result.WriteString(bodyCode)
			result.WriteString(indent + "}\n")
		} else {
			result.WriteString("{}")
		}
	case "method_declaration":
		receiverNode := node.ChildByFieldName("receiver")
		funcNameNode := node.ChildByFieldName("name")
		parametersNode := node.ChildByFieldName("parameters")
		resultTypeNode := node.ChildByFieldName("result")
		bodyNode := node.ChildByFieldName("body")
		result.WriteString(rewriteDeclSignature(funcNameNode, parametersNode, resultTypeNode, receiverNode, sourceCode, indentLevel))
		bodyCode := rewriteDeclBody(bodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		if bodyCode != "" {
			result.WriteString("{\n")
			result.WriteString(bodyCode)
			result.WriteString(indent + "}\n")
		} else {
			result.WriteString("{}")
		}
	case "expression_list":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			childCode := programSlice(child, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
			if childCode != "" {
				result.WriteString(childCode)
			} else {
				result.WriteString("NOT_CARE")
			}
		}
	case "if_statement":
		initNode := node.ChildByFieldName("initializer")
		conditionNode := node.ChildByFieldName("condition")
		bodyNode := node.ChildByFieldName("consequence")
		elseNode := node.ChildByFieldName("alternative")
		rewriteBranchCode := rewriteBranch(initNode, conditionNode, bodyNode, elseNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		result.WriteString(rewriteBranchCode)
	case "return_statement":
		expressionListNode := node.NamedChild(0)
		rewriteReturnCode := rewriteReturn(expressionListNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		if rewriteReturnCode != "" {
			result.WriteString(indent + "return ")
			result.WriteString(rewriteReturnCode)
			result.WriteString("\n")
		}
	case "var_declaration":
		// keep all var declarations that contain the LHS variables of the callee expression
		child := node.NamedChild(0)
		keepVarDecl := false
		for ii := 0; ii < int(child.NamedChildCount()); ii++ {
			inner := child.NamedChild(ii)
			if inner.Type() == "identifier" {
				varName := inner.Content(sourceCode)
				if lhsVars[varName] {
					keepVarDecl = true
					break
					// TODO: use a appending?
				}
			}
		}
		if keepVarDecl {
			result.WriteString(indent + node.Content(sourceCode) + "\n")
		}
	case "assignment_statement":
		sign := "="
		result.WriteString(rewriteExpression(node, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger, sign))
	case "short_var_declaration":
		sign := ":="
		result.WriteString(rewriteExpression(node, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger, sign))
	case "binary_expression":
		opaNode := node.Child(0)
		opNode := node.Child(1)
		opbNode := node.Child(2)

		opaCode := programSlice(opaNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		opbCode := programSlice(opbNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)

		if opaCode != "" && opbCode == "" {
			result.WriteString(opaCode + " " + opNode.Content(sourceCode) + " NOT_CARE ")
		} else if opaCode == "" && opbCode != "" {
			result.WriteString(" NOT_CARE " + opNode.Content(sourceCode) + " " + opbCode)
		} else if opaCode != "" && opbCode != "" {
			result.WriteString(opaCode + " " + opNode.Content(sourceCode) + " " + opbCode)
		}
	case "func_literal":
		funcLitNode := node.Child(1)
		funcReturnNode := node.Child(2)
		bodyCode := programSlice(node.Child(3), sourceCode, calleeFunctionName, lhsVars, indentLevel+1, sugaredLogger)
		if bodyCode != "" {
			result.WriteString("func " + funcLitNode.Content(sourceCode) + " " + funcReturnNode.Content(sourceCode) + " {\n")
			result.WriteString(bodyCode)
			result.WriteString(indent + "}\n")
		}
	case "expression_statement":
		// Only emit if the inner expression is relevant
		inner := node.NamedChild(0)
		if inner != nil {
			innerCode := programSlice(inner, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
			if innerCode != "" {
				result.WriteString(indent + innerCode + "\n")
			}
		}
	case "unary_expression":
		// Preserve constant literal forms like -1, -"x", etc.
		if isLiteralSubtree(node) {
			result.WriteString(node.Content(sourceCode))
			break
		}
		if containsInterestingVariables(node, sourceCode, interestVars) || containsCallExpression(node, sourceCode, calleeFunctionName) {
			result.WriteString(node.Content(sourceCode))
		}
	case "call_expression":
		if containsCallExpression(node, sourceCode, calleeFunctionName) || (containsInterestingVariables(node, sourceCode, interestVars)) {
			result.WriteString(indent + node.Content(sourceCode) + "\n")
		}
	case "for_statement":
		forLoopHead := node.Child(1)
		if forLoopHead != nil {
			// if loop body does not have outbound call or interested variables, remove the whole loop
			bodyNode := node.ChildByFieldName("body")
			bodyCode := rewriteForLoop(node, bodyNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
			if bodyCode != "" {
				// conservatively keep original for loop head
				result.WriteString(indent + "for " + forLoopHead.Content(sourceCode) + " {\n")
				result.WriteString(bodyCode)
				result.WriteString(indent + "}\n")
			}
		}
	case "expression_switch_statement":
		switchHeadNode := node.NamedChild(0)
		switchHeadCode := programSlice(switchHeadNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		switchCasePairs := rewriteSwitch(node, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)

		allSkip := true
		for _, pair := range switchCasePairs {
			if pair.body != "" {
				allSkip = false
				break
			}
		}

		if !allSkip {
			if switchHeadCode != "" {
				result.WriteString(indent + "switch " + switchHeadCode + " {\n")
			} else {
				switchHeadCode = "NOT_CARE"
				result.WriteString(indent + "switch " + switchHeadCode + " {\n")
			}
			for _, pair := range switchCasePairs {
				if pair.body != "" {
					result.WriteString(indent + pair.label)
					result.WriteString(indent + " " + pair.body)
				}
			}
			result.WriteString(indent + "}\n")
		}
	case "go_statement":
		callNode := node.NamedChild(0)
		callCode := programSlice(callNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
		if callCode != "" {
			// handle args (probably a channel, but it is conservative)
			/*
				go call_f(ch, x, y)
				interestVars will conservatively add none chanel var {ch, x, y}
			*/
			result.WriteString(indent + "go " + callCode + "\n")
			goArgsNode := callNode.NamedChild(1)
			for i := 0; i < int(goArgsNode.NamedChildCount()); i++ {
				varName := strings.TrimSpace(goArgsNode.NamedChild(i).Content(sourceCode))
				if varName == "_" {
					continue
				} else {
					interestVars[varName] = true
				}
			}
		}
	case "send_statement":
		channelNode := node.NamedChild(0)
		senderNode := node.NamedChild(1)
		senderCode := programSlice(senderNode, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)

		if senderCode != "" {
			varName := strings.TrimSpace(channelNode.Content(sourceCode))
			if varName != "_" {
				interestVars[varName] = true
			}
			result.WriteString(indent + channelNode.Content(sourceCode) + " <- " + senderCode + "\n")
		}
	case "block":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			childCode := programSlice(child, sourceCode, calleeFunctionName, lhsVars, indentLevel, sugaredLogger)
			if childCode != "" {
				result.WriteString(childCode)
			}
		}
	case "interpreted_string_literal", "int_literal", "float_literal", "char_literal", "bool_literal", ",", "nil", "true", "false":
		result.WriteString(node.Content(sourceCode))
	case "identifier":
		if containsInterestingVariables(node, sourceCode, interestVars) {
			result.WriteString(node.Content(sourceCode))
		}
	default:
		// Skip unhandled nodes silently (debug only) to avoid leaking irrelevant statements
		sugaredLogger.Debug("Node type not handled: ", node.Type())
	}
	return result.String()
}

// Check if the node contains any of the LHS variables
func containsInterestingVariables(node *sitter.Node, sourceCode []byte, lhsVars map[string]bool) bool {
	if node.Type() == "identifier" {
		varName := strings.TrimSpace(node.Content(sourceCode))
		return lhsVars[varName]
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if containsInterestingVariables(child, sourceCode, lhsVars) {
			return true
		}
	}
	return false
}

// isLiteralSubtree returns true if the subtree consists only of literal values (possibly wrapped
// in unary or parenthesized expressions), e.g., -1, (-1), true, "str".
func isLiteralSubtree(node *sitter.Node) bool {
	switch node.Type() {
	case "interpreted_string_literal", "raw_string_literal", "int_literal", "float_literal", "char_literal", "bool_literal", "nil", "true", "false":
		return true
	case "unary_expression", "parenthesized_expression":
		if node.NamedChildCount() == 0 {
			return false
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if !isLiteralSubtree(node.NamedChild(i)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func createIndent(indentLevel int) string {
	return strings.Repeat("  ", indentLevel)
}

type switchCasePair struct {
	label string
	body  string
}
