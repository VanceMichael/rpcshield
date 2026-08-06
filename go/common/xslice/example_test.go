package xslice_test

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/uber-research/rpcshield/go/common/xslice"
)

func ExampleMap() {
	intToStr := strconv.Itoa

	ints := []int{1, 2, 3, 4, 5}
	strings := xslice.Map(ints, intToStr)
	fmt.Println(strings)
	// Output:
	// [1 2 3 4 5]
}

func ExampleTryMap() {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	jsonToStruct := func(jsonString string) (Person, error) {
		var person Person
		err := json.Unmarshal([]byte(jsonString), &person)
		return person, err
	}

	strings := []string{
		`{"name":"name_1","age":54}`,
		`{"name":"name_2","age":31}`,
		`{"name":"name_3","age":12}`,
	}
	persons, err := xslice.TryMap(strings, jsonToStruct)
	if err != nil {
		fmt.Println("got error: ", err)
	} else {
		fmt.Println(persons)
	}
}

func ExampleToMapFunc() {
	type Person struct {
		Name string
		Age  int
	}
	people := []Person{
		{
			Name: "John",
			Age:  25,
		},
		{
			Name: "Jane",
			Age:  26,
		},
	}
	mapped := xslice.ToMapFunc(people, func(p Person) (string, Person) {
		return p.Name, p
	})
	hasPerson := xslice.ToMapFunc(people, func(p Person) (string, bool) {
		return p.Name, true
	})
	fmt.Println(mapped)
	fmt.Println(hasPerson)
	// Output:
	// map[Jane:{Jane 26} John:{John 25}]
	// map[Jane:true John:true]
}

func ExampleFilter() {
	slice := []int{0, 1, 2, 3, 4}
	filtered := xslice.Filter(slice, func(i int) bool {
		return i > 2
	})
	fmt.Printf("original: %v\n", slice)
	fmt.Printf("filtered: %v\n", filtered)
	// Output:
	// original: [0 1 2 3 4]
	// filtered: [3 4]
}

func ExampleFilterInPlace() {
	slice := []int{0, 1, 2, 3, 4}
	filtered := xslice.FilterInPlace(slice, func(i int) bool {
		return i > 2
	})
	fmt.Printf("original: %v\n", slice)
	fmt.Printf("filtered: %v\n", filtered)
	// Output:
	// original: [3 4 0 0 0]
	// filtered: [3 4]
}
