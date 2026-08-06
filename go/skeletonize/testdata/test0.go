package main

import "fmt"

func OutBoundCall() (string, error) {
	fmt.Println("OutBoundCall")
	return "outbound", nil
}

func A() string {
	s := "hello"
	var res string
	if s == "hello" {
	}
	res, err := OutBoundCall(s)
	return s
}

func main() {
	word := A()
	fmt.Println(word)
}
