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

func A() (string, error) {
	s := "hello"
	var res string
	if s == "hello" {
		res, err := OutBoundCall(s)
		if res != nil {
			C(err)
		}
		return res, err
	} else {
		B()
	}
	return s, nil
}

func main() {
	word := A()
	fmt.Println(word)
}
