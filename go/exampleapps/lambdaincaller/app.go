package main

// SomeType is a struct.
type SomeType struct {
	field func()
}

func main() {
	f := func() {}
	StopHere(f)
	StopHere(f)
}

// StopHere is a function.
func StopHere(_ func()) {
	S := SomeType{
		field: func() {
			// Do something
		},
	}
	A(&S)
}

// A is a function.
func A(s *SomeType) {
	B(s.field)
	P(1)
	Q(1)
	NoiseCall(1)
}

// P is a function.
func P(_ int) {}

// Q is a function.
func Q(_ int) {}

// B is a function.
func B(f func()) {
	f()
	P(1)
	Q(1)
	r(1)
}

func r(_ int) {
	// Noise function
}

func f1() {
	// Noise function
}

func f2() {
	// Noise function
}

func f3() {
	// Noise function
}

func f4() {
	// Noise function
}

func f5() {
	// Noise function
}

func f6() {
	// Noise function
}

func f7() {
	// Noise function
}

func f8() {
	// Noise function
}

// NoiseCall is a function.
func NoiseCall(_ int) {
	e := []func(){f1, f2, f3, f4, f5, f6, f7, f8}
	for _, i := range e {
		i() // address taken and invoked.
	}
}
