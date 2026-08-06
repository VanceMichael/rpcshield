package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: comparing FunctionVal lattice with nil ConstantVal works and removes unreachable blocks.

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
	caller(reachableFunc, nil)
}

//go:noinline
func reachableFunc() float32 {
	feasible()
	callSink(1)
	return 1.0
}

func caller(f func() float32, g func() float32) {
	switch {
	case f == nil:
		infeasible()
		callSink(1)
	case g != nil:
		infeasible()
		callSink(1)
	case nil == f:
		infeasible()
		callSink(1)
	case nil != g:
		infeasible()
		callSink(1)
	}
}

func main() {
	source()
}
