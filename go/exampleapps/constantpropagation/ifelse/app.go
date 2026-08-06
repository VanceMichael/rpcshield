package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: comparing ConstantVal passed through functions ensures that infeasible blocks are removed.

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
	v := 42
	callee(v)
}

//go:noinline
func callee(v int) {
	if v > 0 {
		// Only this path is feasible
		feasible()
		callSink(1)
	} else {
		infeasible()
		callSink(1)
	}
}

func main() {
	source()
}
