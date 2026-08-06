package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: constants passed from callers to callees are propagated correctly through switches.

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
	switch v {
	case 0:
		infeasible()
		callSink(1)
	case 1:
		infeasible()
		callSink(1)
	case 2:
		infeasible()
		callSink(1)
	case 42:
		feasible()
		callSink(1)
	default:
		infeasible()
		callSink(1)
	}
}

func main() {
	source()
}
