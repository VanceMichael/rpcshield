package xslice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFoldOnSliceOfIntToInt(t *testing.T) {
	opFunc := func(acc int, i int) int { return acc + i }

	tests := []struct {
		name      string
		giveBase  int
		giveSlice []int
		want      int
	}{
		{
			name:      "empty slice",
			giveSlice: []int{},
			giveBase:  -1,
			want:      -1,
		},
		{
			name:      "nil slice",
			giveBase:  0,
			giveSlice: nil,
			want:      0,
		},
		{
			name:      "normal slice",
			giveBase:  0,
			giveSlice: []int{6, 34, 12, 6, 4, 63},
			want:      125,
		},
		{
			name:      "normal slice with custom base",
			giveBase:  10,
			giveSlice: []int{6, 34, 12, 6, 4, 63},
			want:      135,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Fold(tt.giveSlice, tt.giveBase, opFunc)
			assert.Equal(t, tt.want, actual)
		})
	}
}

func TestFoldOnCustomStructs(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	type Result struct {
		ConcatenatedNames string `json:"name"`
		SumAge            int    `json:"age"`
	}

	opFunc := func(r Result, p Person) Result {
		return Result{
			ConcatenatedNames: r.ConcatenatedNames + "," + p.Name,
			SumAge:            r.SumAge + p.Age,
		}
	}

	tests := []struct {
		name      string
		giveBase  Result
		giveSlice []Person
		want      Result
	}{
		{
			name: "empty slice",
			giveBase: Result{
				ConcatenatedNames: "base",
				SumAge:            -42,
			},
			giveSlice: []Person{},
			want: Result{
				ConcatenatedNames: "base",
				SumAge:            -42,
			},
		},
		{
			name:      "nil slice",
			giveBase:  Result{},
			giveSlice: nil,
			want:      Result{},
		},
		{
			name: "normal slice",
			giveBase: Result{
				ConcatenatedNames: "base",
			},
			giveSlice: []Person{
				{Name: "name_1", Age: 54},
				{Name: "name_2", Age: 31},
				{Name: "name_3", Age: 12},
			},
			want: Result{
				ConcatenatedNames: "base,name_1,name_2,name_3",
				SumAge:            97,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Fold(tt.giveSlice, tt.giveBase, opFunc)
			assert.Equal(t, tt.want, actual)
		})
	}
}
