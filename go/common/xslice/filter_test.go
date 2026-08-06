package xslice

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterInPlace(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		in         []int
		criteria   func(int) bool
		want       []int
		inModified []int
	}{
		{
			name: "nil_safe",
		},
		{
			name:       "even",
			in:         []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			criteria:   func(i int) bool { return i%2 == 0 },
			want:       []int{2, 4, 6, 8, 10},
			inModified: []int{2, 4, 6, 8, 10, 0, 0, 0, 0, 0},
		},
		{
			name:       "odd",
			in:         []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			criteria:   func(i int) bool { return i%2 == 1 },
			want:       []int{1, 3, 5, 7, 9},
			inModified: []int{1, 3, 5, 7, 9, 0, 0, 0, 0, 0},
		},
		{
			name:       "negative",
			in:         []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			criteria:   func(i int) bool { return i < 0 },
			want:       []int{},
			inModified: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:       "positive",
			in:         []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			criteria:   func(i int) bool { return i > 0 },
			want:       []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			inModified: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := FilterInPlace(tt.in, tt.criteria)
			assert.Equal(t, tt.want, out)
			assert.Equal(t, tt.inModified, tt.in)
		})
	}
}

func TestFilter(t *testing.T) {
	tests := map[string]struct {
		in       []int
		criteria func(int) bool
		want     []int
	}{
		"nils": {},
		"even": {
			in:       []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			criteria: func(i int) bool { return i%2 == 0 },
			want:     []int{2, 4, 6, 8, 10},
		},
		"odd": {
			in:       []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			criteria: func(i int) bool { return i%2 == 1 },
			want:     []int{1, 3, 5, 7, 9},
		},
		"negative": {
			in:       []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			criteria: func(i int) bool { return i < 0 },
			want:     []int{},
		},
		"positive": {
			in:       []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			criteria: func(i int) bool { return i > 0 },
			want:     []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var original []int
			if tt.in != nil {
				original = make([]int, len(tt.in))
				copy(original, tt.in)
			}
			out := Filter(tt.in, tt.criteria)
			assert.Equal(t, tt.want, out)
			assert.Equal(t, original, tt.in, "original slice should not be modified")
		})
	}
}

func BenchmarkFilter(b *testing.B) {
	tests := []struct {
		giveLen int
	}{
		{10},
		{50},
		{100},
	}
	for _, tt := range tests {
		b.Run(strconv.Itoa(tt.giveLen), func(b *testing.B) {
			in := make([]int, tt.giveLen)
			for i := range in {
				in[i] = i
			}
			for i := 0; i < b.N; i++ {
				_ = Filter(in, func(i int) bool {
					return i%2 == 0
				})
			}
		})
	}
}
