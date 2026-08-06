package input

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestNewLLMFuncInput(t *testing.T) {
	got := NewLLMFuncInput("foo", "foo.go", 1, 5)
	want := &LLMFuncInput{FuncName: "foo", FilePath: "foo.go", StartLine: 1, EndLine: 5}
	if *got != *want {
		t.Errorf("NewLLMFuncInput() = %+v, want %+v", got, want)
	}
}

func TestNewLLMAnalyzerInput(t *testing.T) {
	caller := NewLLMFuncInput("foo", "foo.go", 1, 5)
	got := NewLLMAnalyzerInput(caller, "bar", 3)
	if got.Caller != caller || got.Callee != "bar" || got.CalleeSitePos != 3 {
		t.Errorf("NewLLMAnalyzerInput() = %+v", got)
	}
}

func TestNewStaticAnalyzerInput(t *testing.T) {
	var caller, callee ast.Node
	got := NewStaticAnalyzerInput(caller, callee, token.Pos(42), nil)
	if got.CalleeSitePos != token.Pos(42) {
		t.Errorf("NewStaticAnalyzerInput().CalleeSitePos = %v, want %v", got.CalleeSitePos, 42)
	}
}
