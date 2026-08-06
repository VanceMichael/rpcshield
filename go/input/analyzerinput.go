package input

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/packages"
)

// LLMFuncInput stores the input for a function to be analyzed by LLM
type LLMFuncInput struct {
	FuncName  string
	FilePath  string
	StartLine int
	EndLine   int
}

// NewLLMFuncInput creates a new instance of the LLMFuncInput struct
func NewLLMFuncInput(funcName, filePath string, startLine, endLine int) *LLMFuncInput {
	return &LLMFuncInput{
		FuncName:  funcName,
		FilePath:  filePath,
		StartLine: startLine,
		EndLine:   endLine,
	}
}

// LLMAnalyzerInput stores the input for the LLM analyzer
type LLMAnalyzerInput struct {
	Caller        *LLMFuncInput
	Callee        string
	CalleeSitePos int // Line number of the callee site in the caller
}

// NewLLMAnalyzerInput creates a new instance of the LLMAnalyzerInput struct
func NewLLMAnalyzerInput(caller *LLMFuncInput, callee string, calleeSitePos int) *LLMAnalyzerInput {
	return &LLMAnalyzerInput{
		Caller:        caller,
		Callee:        callee,
		CalleeSitePos: calleeSitePos,
	}
}

// StaticAnalyzerInput stores the input for the static analyzer
type StaticAnalyzerInput struct {
	Caller        ast.Node
	Callee        ast.Node
	CalleeSitePos token.Pos // Position of the callee site in the caller
	Pkgs          []*packages.Package
}

// NewStaticAnalyzerInput creates a new instance of the StaticAnalyzerInput struct
func NewStaticAnalyzerInput(caller, callee ast.Node, calleeSitePos token.Pos, pkgs []*packages.Package) *StaticAnalyzerInput {
	return &StaticAnalyzerInput{
		Caller:        caller,
		Callee:        callee,
		CalleeSitePos: calleeSitePos,
		Pkgs:          pkgs,
	}
}
