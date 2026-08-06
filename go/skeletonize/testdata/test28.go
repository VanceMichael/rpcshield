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

func C(res string) {
	return
}

func A() (string, error, int) {
	s := "hello"
	var res string
	res, _ = OutBoundCall(s)
	t := res + s + "" + -1
	return s, nil, 2
}

func main() {
	word := A()
	fmt.Println(word)
}
