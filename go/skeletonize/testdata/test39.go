package main

import "fmt"

func OutBoundCall() (string, error) {
	fmt.Println("OutBoundCall")
	return "outbound", nil
}

func A() string {
	s := "hello"
	ch := make(chan error)
	res, err := OutBoundCall(s)
	ch <- err
	value := <-ch
	return s
}

func main() {
	word := A()
	fmt.Println(word)
}
