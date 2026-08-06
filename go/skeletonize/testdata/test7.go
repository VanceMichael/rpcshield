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
	res, err := OutBoundCall(s)

	if s == "hello" {
		res, err := B(res)
		if res != nil {
			C(err)
		} else {
			q := "world"
			B(q)
		}
		return res, err
	} else {
		B()
	}

	for i := 0; i < 10; i++ {
		B()
	}
	return s, nil
}
