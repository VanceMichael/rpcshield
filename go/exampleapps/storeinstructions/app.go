package main

// MyStruct is a struct with a field.
type MyStruct struct {
	Field int
}

// CodeUnderTest is a function with some code to analyze.
func CodeUnderTest() {
	var arr [5]int
	var x, y int
	var s MyStruct

	x = 42
	y = arr[1]  // IndexAddr
	s.Field = x // FieldAddr
	_ = y
}
func main() {}
