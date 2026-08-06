package xslice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnique(t *testing.T) {
	tests := []struct {
		name string
		give []int
		want []int
	}{
		{
			name: "nil input",
			give: nil,
			want: nil,
		},
		{
			name: "empty slice",
			want: []int{},
			give: []int{},
		},
		{
			name: "slice with single element",
			give: []int{1},
			want: []int{1},
		},
		{
			name: "slice with no duplicates",
			give: []int{1, 2, 3, 4, 5},
			want: []int{1, 2, 3, 4, 5},
		},
		{
			name: "slice with duplicates",
			give: []int{1, 4, 4, 4, 2, 2, 2, 2, 2, 3, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5},
			want: []int{1, 4, 2, 3, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Unique(tt.give)
			require.Equal(t, tt.want, got)

			// If the input and output lengths match, no elements have been removed
			// and thus the input should be returned verbatim.
			if len(tt.want) > 0 && len(tt.want) == len(tt.give) {
				if &tt.give[0] != &got[0] {
					t.Error("input is not returned verbatim")
					return
				}
			}
		})
	}
}

func TestAllMatch(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  bool
	}{
		{
			name:  "nil input",
			input: nil,
			want:  true,
		},
		{
			name:  "empty slice",
			input: []int{},
			want:  true,
		},
		{
			name:  "all positive values",
			input: []int{1, 2, 3, 4, 5},
			want:  true,
		},
		{
			name:  "all negative values",
			input: []int{-1, -2, -3, -4, -5},
			want:  false,
		},
		{
			name:  "mixed values",
			input: []int{1, -2, 3, -4, 5},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AllMatch(tt.input, func(x int) bool {
				return x > 0
			})
			assert.Equal(t, tt.want, got)
		})
	}
}
