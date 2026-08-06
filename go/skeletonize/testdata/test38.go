package main

import "fmt"

func OutBoundCall() (string, error) {
	fmt.Println("OutBoundCall")
	return "outbound", nil
}

func A() string {
	s := "hello"
	go d()
	go OutBoundCall(s)
	return s
}

func main() {
	word := A()
	fmt.Println(word)
}
