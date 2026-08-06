package main

import "fmt"

func OutBoundCall() (string, error) {
	fmt.Println("OutBoundCall")
	return "outbound", nil
}

func A() string {
	s := "hello"
	var res string
	day := 3
	switch day {
	case 1:
		fmt.Println("Monday")
	case 2:
		fmt.Println("Tuesday")
	default:
		res, err := OutBoundCall(s)
	}

	res, err := OutBoundCall(s)
	return s
}

func main() {
	word := A()
	fmt.Println(word)
}
