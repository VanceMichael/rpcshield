package main

import (
	_ "context"
	_ "sync"
)

// AA defines an interface
type AA interface {
	Func1()
	Func2()
}

// B is a struct
type B struct {
}

// Func1 is a method
func (b *B) Func1() {
}

// Func2 is a method
func (b *B) Func2() {
}

func caller(a AA) {
	a.Func1()
}

func main() {
}
