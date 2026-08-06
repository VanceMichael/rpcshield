package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: comparing FunctionVal lattice for ssa.MakeClosure works and removes infeasible indirect invokes.

//go:noinline
func sink() {
}

//go:noinline
func infeasible() {}

//go:noinline
func feasible() {}

//go:noinline
func callSink(v int) {
	sink()
}

//go:noinline
func source() {
	caller(func() float32 {
		return reachableFunc(1, 2, 3, 4)
	})
}

//go:noinline
func reachableFunc(a, b, c, d int) float32 {
	feasible()
	callSink(1)
	return 1.0
}

//go:noinline
func unreachableFunc(a, b, c, d int) float32 {
	infeasible()
	callSink(1)
	return 1.0
}

//go:noinline
func instantiate(v int) func() float32 {
	// These instantiations create types resulting RTA callgraph to add infeasible edges.
	return func() float32 {
		infeasible()
		callSink(1)
		return unreachableFunc(1, 2, 3, 4)
	}
}

func caller(f func() float32) {
	_ = f()
}

// Only path main -> source -> caller -> reachableFunc() -> sink() is feasible.
// main -> source -> caller -> unreachableFunc() -> sink() is infeasible
func main() {
	source()
	_ = instantiate(0)
}
