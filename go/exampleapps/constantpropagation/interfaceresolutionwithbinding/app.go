package main

// If a path from source to sink has an infeasible() call in the same basic block, as the call site, the path should be infeasible.

// Test objective: passing the correct receiver object disambiguates between the two types implementing the interface even when the methods are invoked by binding receivers.

// Iface is an interface.
type Iface interface {
	Foo(float32)
}

//go:noinline
func sink() {}

//go:noinline
func infeasible() {}

//go:noinline
func feasible() {}

//go:noinline
func callSink(v int) {
	sink()
}

// B is a concrete type that implements Iface.
type B struct{}

// Foo is a method that implements Iface.
//
//go:noinline
func (b *B) Foo(_ float32) {
	infeasible()
	callSink(1)
}

// C is a concrete type that implements Iface.
type C struct{}

// Foo is a method that implements Iface.
//
//go:noinline
func (c *C) Foo(_ float32) {
	feasible()
	callSink(1)
}

//go:noinline
func callee2(v func(float32)) {
	v(42.0)
}

//go:noinline
func callee1(a Iface) {
	v := a.Foo
	callee2(v)
}

//go:noinline
func source() {
	var a Iface
	a = &C{}
	callee1(a)
}

//go:noinline
func addressTakenFunc() {
	infeasible()
	callSink(1)
}

//go:noinline
func instantiateOtherTypes() (Iface, func()) {
	// These instantiations create types resulting RTA callgraph to add infeasible edges.
	var a Iface
	a = &B{}
	return a, addressTakenFunc
}

func main() {
	source()
	_, _ = instantiateOtherTypes()
}
