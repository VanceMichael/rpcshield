package main

import "fmt"

// Scenario 1: Dynamic dispatch to method, method has a pointer receiver

// Iface1 defines an interface for testing dynamic dispatch with value receivers.
type Iface1 interface {
	Foo()
}

// Impl1 implements Iface1 for testing synthetic function unwrapping.
type Impl1 struct{}

// Foo implements the Iface1 interface with a value receiver.
func (i Impl1) Foo() {}

func work1(arg Iface1) {
	arg.Foo()
}

// Scenario 2: Dynamic dispatch to method, method has a value receiver

// Iface2 defines an interface for testing dynamic dispatch with value receivers.
type Iface2 interface {
	Foo()
}

// Impl2 implements Iface2 for testing synthetic function unwrapping.
type Impl2 struct{}

// Foo implements the Iface2 interface with a value receiver.
func (i Impl2) Foo() {}

func work2(arg Iface2) {
	arg.Foo()
}

// Scenario 3: Generic function
func work3[T any](arg T) {
	fmt.Println(arg)
}

// Scenario 4: Generic struct, static + dynamic dispatch

// Impl3 is a generic struct for testing synthetic function unwrapping with generics.
type Impl3[T any] struct {
	Value T
}

func (i Impl3[T]) Foo() {
	fmt.Println(i.Value)
}

func (i *Impl3[T]) FooPtr() {
	fmt.Println(i.Value)
}

func work4[T any](arg *Impl3[T]) {
	arg.Foo()
	arg.FooPtr()
}

func main() {
	// Scenario 1
	arg1 := Impl1{}
	work1(arg1)

	// Scenario 2
	arg2 := Impl2{}
	work2(arg2)

	// Scenario 3
	work3(arg1)
	work3(arg2)

	// Scenario 4
	arg4 := &Impl3[int]{Value: 1}
	work4(arg4)
}
