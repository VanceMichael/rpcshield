package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: channel receive with comma-ok pattern creates tuple lattice elements correctly.

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
	channelReceiveCommaOk()
	channelReceiveSimple()
}

// Test objective: channel receive with comma-ok pattern correctly handles tuple extraction.
//
//go:noinline
func channelReceiveCommaOk() {
	ch := make(chan int, 1)
	ch <- 42
	v, ok := <-ch
	// Since channel receive is bottom, both branches are feasible.
	if ok {
		feasible()
		callSink(v)
	} else {
		feasible()
		callSink(v)
	}
	// Since v is bottom, both branches are feasible.
	if v == 42 {
		feasible()
		callSink(v)
	} else {
		feasible()
		callSink(v)
	}
}

// Test objective: simple channel receive (without comma-ok) also works correctly.
//
//go:noinline
func channelReceiveSimple() {
	ch := make(chan int, 1)
	ch <- 42
	v := <-ch
	// Since channel receive is bottom, both branches are feasible.
	if v == 42 {
		feasible()
		callSink(v)
	} else {
		feasible()
		callSink(v)
	}
}

func main() {
	source()
}
