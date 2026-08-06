package cfgfuncs

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
)

var cond bool

type myErr struct{}

func (myErr) Error() string { return "myErr message" }

type S struct{}

func retPtrErr() (*S, error) {
	if cond {
		return nil, errors.New("error")
	}
	return &S{}, nil
}

func func1() (*S, error) {
	s, err := retPtrErr()
	if err != nil {
		return nil, err
	}
	return s, err
}

func func2() (*S, error) {
	s, err := retPtrErr()
	if err != nil {
		return nil, errors.New("error")
	}
	return s, nil
}

func func3() (*S, error) {
	s, err := retPtrErr()
	return s, fmt.Errorf("error %w", err)
}

func func4() (*S, error) {
	s, _ := retPtrErr()
	return s, nil
}

func func5() {
	_, _ = retPtrErr()
}

func testSimpleConditional(n int) string {
	if n > 0 {
		return "Positive"
	} else if n < 0 {
		return "Negative"
	} else {
		return "Zero"
	}
}

func testLoop(max int) int {
	sum := 0
	for i := 1; i <= max; i++ {
		if i%2 == 0 {
			continue
		}
		sum += i
	}
	return sum
}

func testNestedLoops(n int) int {
	count := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			count++
		}
	}
	return count
}

func testRecursion(n int) int {
	if n == 0 {
		return 1
	}
	return n * testRecursion(n-1)
}

func testSwitch(dayNum int) string {
	switch dayNum {
	case 1:
		return "Sunday"
	case 2:
		return "Monday"
	case 3:
		return "Tuesday"
	case 4:
		return "Wednesday"
	case 5:
		return "Thursday"
	case 6:
		return "Friday"
	case 7:
		return "Saturday"
	default:
		return "Invalid day"
	}
}

type I interface {
	foo(x *A) (*A, string)
}

type J interface {
	bar(x *A, y *B) *string
}

type A struct {
	s string
}

type B struct {
	i int
}

func (A) foo(x *A) (*A, string) {
	var b *A
	return b, x.s
}

func (a A) bar(x *A, y *B) *string {
	if x != nil {
		return &x.s
	}
	return &a.s
}

func (b B) foo(x *A) (*A, string) {
	return x, x.s
}

func (b *B) bar(x *A, y *B) *string {
	if b.i+y.i > 5 {
		return nil
	}
	return &x.s
}

// neverReturns has no reachable return statement, so buildDecl marks it as a
// no-return function.
func neverReturns() {
	for {
	}
}

func testInterfaceCalls() {
	var v1 I
	v1 = &A{}
	v1.foo(new(A))

	var v2 J
	v2 = &B{}
	v2.bar(new(A), new(B))
}

func errReturn(a bool) (*int, error) {
	i := 42
	if a {
		return &i, nil
	}
	return nil, errors.New("some error")
}

func testAbnormalControlFlow(msg string, val bool) {
	ptr, err := errReturn(val)
	switch msg {
	case "print":
		if err != nil {
			print(err)
		}
		print(*ptr)
	case "print_and_return":
		if err != nil {
			print(err)
			return
		}
		print(*ptr)
	case "panic":
		if err != nil {
			panic(err)
		}
		print(*ptr)
	case "log.Fatal":
		if err != nil {
			log.Fatal(err)
		}
		print(*ptr)
	case "log.Fatalf":
		if err != nil {
			log.Fatalf("msg %s", err)
		}
		print(*ptr)
	case "os.Exit":
		if err != nil {
			os.Exit(1)
		}
		print(*ptr)
	case "runtime.Goexit":
		if err != nil {
			runtime.Goexit()
		}
		print(*ptr)
	}
}
