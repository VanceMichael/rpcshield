package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: passing the correct receiver object disambiguates between the two types implementing the interface.

// Iface is an interface.
type Iface interface {
	Foo()
}

//go:noinline
func sink() {
}

//go:noinline
func callSink(v int) {
	sink()
}

//go:noinline
func infeasible() {}

//go:noinline
func feasible() {}

// B is a concrete type that implements Iface.
type B struct{}

// Foo is a method that implements Iface.
//
//go:noinline
func (b *B) Foo() {
	infeasible()
	callSink(1)
}

// C is a concrete type that implements Iface.
type C struct{}

// Foo is a method that implements Iface.
//
//go:noinline
func (c *C) Foo() {
	feasible()
	callSink(1)
}

//go:noinline
func source() {
	var a Iface
	a = &C{}
	caller(a)
}

//go:noinline
func instantiateB() Iface {
	// Instantiation of B creates a type resulting RTA callgraph to add infeasible edges.
	var a Iface
	a = &B{}
	return a
}

func caller(a Iface) {
	a.Foo()
}

func main() {
	source()
	_ = instantiateB()
}
