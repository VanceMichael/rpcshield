package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: phi instuctions correctly model incoming arguments to infer constantness.

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

//go:noinline
func source() {
	callee1(42)
}

//go:noinline
func callee1(val int) {
	f1 := func(float32) {
		println("iam f1")
		feasible()
		callSink(1)
	}
	f2 := func(float32) {
		println("iam f2")
		infeasible()
		callSink(1)
	}
	f3 := func(float32) {
		println("iam f3")
		infeasible()
		callSink(1)
	}
	var f func(float32)
	switch val {
	case 42:
		f = f1
	case 1:
		f = f2
	default:
		f = f3
	}
	f(1.0) // The phi here resolves to only f1 compatible path.
	callee2(f, f, f)
}

var myrandVal int = 42

//go:noinline
func myrand() int {
	return ((myrandVal + 1) * myrandVal) % 100
}

//go:noinline
func callee2(f1, f2, f3 func(float32)) {
	val := myrand()
	var f func(float32)
	switch val {
	case 42:
		f = f1
	case 1:
		f = f2
	default:
		f = f3
	}
	f(1.0) // The phi here resolves to only f1 compatible path.
}

func main() {
	source()
}
