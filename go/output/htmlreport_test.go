package output

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/callnode"
)

func TestBuildDotGraph(t *testing.T) {
	// Create mock Data
	mockInboundFuncBlock := callnode.FuncBlock{
		Function:  "mockInboundFunction",
		File:      "/path/to/mockFile.go",
		StartLine: 100,
		EndLine:   259,
	}
	mockCallerFuncBlock := callnode.FuncBlock{
		Function:  "mockCallerFunction",
		File:      "/path/to/mockFile2.go",
		StartLine: 10,
		EndLine:   20,
	}
	mockCalleeFuncBlock := callnode.FuncBlock{
		Function:  "mockCalleeFunction",
		File:      "/path/to/mockFile3.go",
		StartLine: 30,
		EndLine:   40,
	}
	mockCallerCallee := callnode.Node{
		FuncBlock:   mockCallerFuncBlock,
		Callsite:    150,
		Status:      []string{"fail-close"},
		Explanation: []string{"some explanation"},
		Fcalls: []callnode.Node{
			{
				FuncBlock: mockCalleeFuncBlock,
				Callsite:  15,
			},
		},
	}
	mockSection := callnode.Section{
		FuncBlock:   mockInboundFuncBlock,
		Status:      []string{"fail-close"},
		Explanation: []string{"some explanation"},
		Fcalls:      []callnode.Node{mockCallerCallee},
	}
	mockData := callnode.Data{
		Sections: []callnode.Section{mockSection},
	}

	result := buildDotGraph(mockData)
	require.Len(t, result, 1)
	require.True(t, strings.Contains(result[0], "\"mockInboundFunction_1\" -> \"mockCallerFunction_1\""))
	require.True(t, strings.Contains(result[0], "\"mockCallerFunction_1\" -> \"mockCalleeFunction_1\""))
}
