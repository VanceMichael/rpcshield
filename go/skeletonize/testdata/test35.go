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
	case 9:
		fmt.Println("Sunday")
	case 2:
		fmt.Println("Monday")
	default:
		fmt.Println("Tuesday")
	}

	res, err := OutBoundCall(s)
	return s
}

func main() {
	word := A()
	fmt.Println(word)
}
