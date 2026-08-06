package callnode

import (
	"fmt"
	"strings"
)

// FuncBlock is a struct that represents a function block
type FuncBlock struct {
	Function  string `json:"function"`
	File      string `json:"file"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
}

// Node is a struct that represents a function call
type Node struct {
	FuncBlock
	Callsite    int      `json:"callsite"`
	Status      []string `json:"status,omitempty"`
	Explanation []string `json:"explanation,omitempty"`
	Fcalls      []Node   `json:"callees,omitempty"`
	Parent      *Node    `json:"-"`
	TopSection  *Section `json:"-"`
}

// Section is a struct that represents a section (a single call tree) of the json file
type Section struct {
	FuncBlock
	Status      []string `json:"status,omitempty"`
	Explanation []string `json:"explanation,omitempty"`
	Fcalls      []Node   `json:"callees,omitempty"`
}

// Data is a struct that represents the json file
type Data struct {
	Meta     map[string]interface{} `json:"meta"`
	Sections []Section              `json:"sections"`
}

// Helper function for recursive string building
func (f Node) buildString(level int) string {
	indent := strings.Repeat("  ", level)
	result := fmt.Sprintf("%sFcall Function: %s\n", indent, f.FuncBlock.Function)
	result += fmt.Sprintf("%sFcall File: %s\n", indent, f.FuncBlock.File)
	result += fmt.Sprintf("%sFcall SLine: %d\n", indent, f.FuncBlock.StartLine)
	result += fmt.Sprintf("%sFcall ELine: %d\n", indent, f.FuncBlock.EndLine)
	result += fmt.Sprintf("%sFcall Callsite: %d\n", indent, f.Callsite)
	result += fmt.Sprintf("%sFcall Status: %s\n", indent, f.Status)
	for _, subFcall := range f.Fcalls {
		result += subFcall.buildString(level + 1)
	}
	return result
}

// Implement the String method for the Fcall struct
func (f Node) String() string {
	return f.buildString(0)
}

func (s Section) String() string {
	return fmt.Sprintf("Section Function: %s\nSection File: %s\nSection SLine: %d\n"+
		"Section ELine: %d\nSection Status: %s\nSection Explanation: %s\n Section Fcalls: %v\n",
		s.FuncBlock.Function, s.FuncBlock.File, s.FuncBlock.StartLine, s.FuncBlock.EndLine, s.Status, s.Explanation, s.Fcalls)
}
