package main

import (
	"three"
	"two"
)

//go:noinline
func funclit(a, b int) int {
	f := func() int {
		return 42
	}
	v1 := f()
	v2 := three.SomeVar
	return v1 + v2
}

func main() {
	v1 := funclit(42, 24)
	v2 := two.Two()
	_ = v1 + v2
}
