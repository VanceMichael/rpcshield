package main

import "fmt"

func OutBoundCall() (string, error) {
	fmt.Println("OutBoundCall")
	return "outbound", nil
}

func B() {
	fmt.Println("B")
	return
}

func A() (string, error) {
	s := "hello"
	var res string
	if s == "hello" {
		B()
	} else {
		res, err := OutBoundCall(s)
		return res, err
	}
	return s, nil
}

func main() {
	word := A()
	fmt.Println(word)
}
