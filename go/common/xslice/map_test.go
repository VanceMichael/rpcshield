package xslice

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapOnSliceOfIntToString(t *testing.T) {
	mapFunc := strconv.Itoa

	tests := []struct {
		name      string
		giveSlice []int
		wantSlice []string
	}{
		{
			name:      "empty slice",
			giveSlice: []int{},
			wantSlice: []string{},
		},
		{
			name:      "nil slice",
			giveSlice: nil,
			wantSlice: nil,
		},
		{
			name:      "normal slice",
			giveSlice: []int{6, 34, 12, 6, 4, 63},
			wantSlice: []string{"6", "34", "12", "6", "4", "63"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSlice := Map(tt.giveSlice, mapFunc)
			assert.Equal(t, tt.wantSlice, actualSlice)
		})
	}
}

func TestMapOnCustomStructs(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	mapFunc := func(p Person) string {
		return fmt.Sprintf("%s:%d", p.Name, p.Age)
	}

	tests := []struct {
		name             string
		giveSlice        []Person
		wantSlice        []string
		wantErrorMessage string
	}{
		{
			name:      "empty slice",
			giveSlice: []Person{},
			wantSlice: []string{},
		},
		{
			name:      "nil slice",
			giveSlice: nil,
			wantSlice: nil,
		},
		{
			name: "normal slice",
			giveSlice: []Person{
				{Name: "name_1", Age: 54},
				{Name: "name_2", Age: 31},
				{Name: "name_3", Age: 12},
			},
			wantSlice: []string{
				"name_1:54",
				"name_2:31",
				"name_3:12",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSlice := Map(tt.giveSlice, mapFunc)
			assert.Equal(t, tt.wantSlice, actualSlice)
		})
	}
}

func TestTryMapOnSliceOfIntToString(t *testing.T) {
	mapFunc := func(i int) (string, error) {
		return strconv.Itoa(i), nil
	}

	tests := []struct {
		name      string
		giveSlice []int
		wantSlice []string
	}{
		{
			name:      "empty slice",
			giveSlice: []int{},
			wantSlice: []string{},
		},
		{
			name:      "nil slice",
			giveSlice: nil,
			wantSlice: nil,
		},
		{
			name:      "normal slice",
			giveSlice: []int{6, 34, 12, 6, 4, 63},
			wantSlice: []string{"6", "34", "12", "6", "4", "63"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSlice, err := TryMap(tt.giveSlice, mapFunc)
			assert.Equal(t, tt.wantSlice, actualSlice)
			assert.NoError(t, err)
		})
	}
}

func TestTryMapOnCustomStructs(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	type expectedError struct {
		errMsg  string
		errType error
	}

	mapFunc := func(jsonString string) (*Person, error) {
		var person Person
		err := json.Unmarshal([]byte(jsonString), &person)
		if err != nil {
			return nil, err
		}
		return &person, nil
	}

	tests := []struct {
		name      string
		giveSlice []string
		wantSlice []*Person
		wantError *expectedError
	}{
		{
			name:      "empty slice",
			giveSlice: []string{},
			wantSlice: []*Person{},
		},
		{
			name:      "nil slice",
			giveSlice: nil,
			wantSlice: nil,
		},
		{
			name: "json unmarshall error on the 3rd item",
			giveSlice: []string{
				`{"name":"name_1","age":54}`,
				`{"name":"name_2","age":31}`,
				"can't unrmarshal this",
			},
			wantSlice: nil,
			wantError: &expectedError{
				errMsg:  "trymap: element at '2', error: invalid character 'c' looking for beginning of value",
				errType: &json.SyntaxError{},
			},
		},
		{
			name: "success",
			giveSlice: []string{
				`{"name":"name_1","age":54}`,
				`{"name":"name_2","age":31}`,
				`{"name":"name_3","age":12}`,
			},
			wantSlice: []*Person{
				{Name: "name_1", Age: 54},
				{Name: "name_2", Age: 31},
				{Name: "name_3", Age: 12},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSlice, err := TryMap(tt.giveSlice, mapFunc)
			assert.Equal(t, tt.wantSlice, actualSlice)

			if tt.wantError == nil {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.wantError.errMsg, err.Error())

				wantErrType := reflect.New(reflect.TypeOf(tt.wantError.errType)).Interface()
				assert.ErrorAs(t, err, wantErrType)
			}
		})
	}
}

func TestToMapFunc(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}
	john := &Person{
		Name: "John",
		Age:  25,
	}
	jane := &Person{
		Name: "Jane",
		Age:  26,
	}
	john2 := &Person{
		Name: "John",
		Age:  27,
	}
	t.Run("success", func(t *testing.T) {
		people := []*Person{
			john,
			jane,
		}
		nameToPerson := ToMapFunc(people, func(person *Person) (string, *Person) {
			return person.Name, person
		})
		assert.Equal(t, nameToPerson["John"], john)
		assert.Equal(t, nameToPerson["Jane"], jane)
	})
	t.Run("has_name", func(t *testing.T) {
		people := []*Person{
			john,
			jane,
		}
		hasName := ToMapFunc(people, func(person *Person) (string, bool) {
			return person.Name, true
		})
		assert.True(t, hasName["John"])
		assert.True(t, hasName["Jane"])
		assert.False(t, hasName["James"])
	})
	t.Run("key_collision", func(t *testing.T) {
		people := []*Person{
			john,
			jane,
			john2,
		}
		nameToPerson := ToMapFunc(people, func(person *Person) (string, *Person) {
			return person.Name, person
		})
		assert.Len(t, nameToPerson, 2)
		assert.NotEqual(t, nameToPerson["John"], john)
		assert.Equal(t, nameToPerson["John"], john2)
	})
}
