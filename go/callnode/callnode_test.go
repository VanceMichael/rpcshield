package callnode

import (
	"strings"
	"testing"
)

func TestNodeString(t *testing.T) {
	n := Node{
		FuncBlock: FuncBlock{Function: "foo", File: "foo.go", StartLine: 1, EndLine: 5},
		Callsite:  10,
		Status:    []string{"fail-close"},
		Fcalls: []Node{
			{FuncBlock: FuncBlock{Function: "bar", File: "bar.go", StartLine: 2, EndLine: 3}},
		},
	}

	got := n.String()
	for _, want := range []string{"Fcall Function: foo", "Fcall Callsite: 10", "  Fcall Function: bar"} {
		if !strings.Contains(got, want) {
			t.Errorf("Node.String() = %q, want substring %q", got, want)
		}
	}
}

func TestSectionString(t *testing.T) {
	s := Section{
		FuncBlock: FuncBlock{Function: "handler", File: "handler.go", StartLine: 1, EndLine: 20},
		Status:    []string{"fail-open"},
	}

	got := s.String()
	if !strings.Contains(got, "Section Function: handler") {
		t.Errorf("Section.String() = %q, want substring %q", got, "Section Function: handler")
	}
}
