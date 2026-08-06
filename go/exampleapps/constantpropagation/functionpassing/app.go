package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: comparing FunctionVal lattice with other FunctionVal/ConstantVal works and removed infeasible blocks.

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

//go:noinline
func source() {
	caller(reachableFunc)
	funcCompare(func1, func2)
}

func func1() string {
	return "foo"
}

func func2() string {
	return "bar"
}

func funcCompare(f1, f2 any) {
	switch {
	case f1 == f2:
		infeasible()
		callSink(1)
	case f1 != f1:
		infeasible()
		callSink(1)
	case f1 == nil:
		infeasible()
		callSink(1)
	case f2 == nil:
		infeasible()
		callSink(1)
	}
}

//go:noinline
func reachableFunc() float32 {
	feasible()
	callSink(1)
	return 1.0
}

//go:noinline
func unreachableFunc() float32 {
	infeasible()
	callSink(1)
	return 1.0
}

//go:noinline
func instantiate() func() float32 {
	return unreachableFunc
}

func caller(f func() float32) {
	_ = f()
}

func main() {
	source()
	_ = instantiate()
}
