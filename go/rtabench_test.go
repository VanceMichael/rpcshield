package rpcshield

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/graph"
)

func TestParseRTAFlavors(t *testing.T) {
	allFlavors := graph.AllFlavors

	tests := []struct {
		name     string
		input    string
		expected []graph.RTAFlavor
		wantErr  bool
	}{
		{name: "empty returns all", input: "", expected: allFlavors},
		{name: "whitespace returns all", input: "  ", expected: allFlavors},
		{name: "all keyword", input: "all", expected: allFlavors},
		{name: "single flavor", input: "srta", expected: []graph.RTAFlavor{graph.FlavorSRTA}},
		{name: "multiple flavors", input: "srta,prta_naive", expected: []graph.RTAFlavor{graph.FlavorSRTA, graph.FlavorPRTANaive}},
		{name: "with spaces", input: " srta , prta_naive ", expected: []graph.RTAFlavor{graph.FlavorSRTA, graph.FlavorPRTANaive}},
		{name: "trailing comma", input: "srta,", expected: []graph.RTAFlavor{graph.FlavorSRTA}},
		{name: "all known flavors listed", input: "srta,srta_kumo,stdlib,cha", expected: []graph.RTAFlavor{graph.FlavorSRTA, graph.FlavorSRTAKumo, graph.FlavorStdlib, graph.FlavorCHA}},
		{name: "invalid flavor", input: "invalid_flavor", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := graph.ParseRTAFlavors(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseRTANumWorkers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
		wantErr  bool
	}{
		{name: "empty returns default", input: "", expected: []int{64}},
		{name: "whitespace returns default", input: "  ", expected: []int{64}},
		{name: "single value", input: "8", expected: []int{8}},
		{name: "multiple values", input: "1,4,16", expected: []int{1, 4, 16}},
		{name: "with spaces", input: " 2 , 8 ", expected: []int{2, 8}},
		{name: "trailing comma", input: "8,", expected: []int{8}},
		{name: "invalid errors", input: "abc", wantErr: true},
		{name: "zero errors", input: "0", wantErr: true},
		{name: "negative errors", input: "-1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := graph.ParseRTANumWorkers(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
