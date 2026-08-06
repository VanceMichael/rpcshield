package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/uber-research/rpcshield/go/features/safego"
)

var crazyF func(ctx context.Context, input float64) (int32, error)

func flipdoor(sw bool) {
	if sw {
		crazyF = func(ctx context.Context, input float64) (int32, error) {
			return int32(10), fmt.Errorf("True-error")
		}
	} else {
		crazyF = func(ctx context.Context, input float64) (int32, error) {
			return int32(10), fmt.Errorf("Fake-error")
		}
	}
}

func generalFCall(ctx context.Context, input float64) (int32, error) {
	return int32(10), fmt.Errorf("Fake-error")
}

func generalNilECall(ctx context.Context, input float64) (int32, error) {
	return int32(10), nil
}

func generalTCall(ctx context.Context, input float64) (int32, error) {
	return int32(10), fmt.Errorf("True-error")
}

func notContextCall(input float64) (int32, error) {
	return int32(8), nil
}

var globalErrChan chan error

//go:noinline
func staticFunc() {
	_, err := generalNilECall(context.Background(), 21.3)
	if err != nil {
		fmt.Println("Error:", err)
	}
	globalErrChan <- err
}

// Response is a struct that represents the response of a function.
type Response struct {
	Success bool
}

// AFunctionWithUniqueName synchronously creates and saves a new SafeGo demo.
func AFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	//random := rand.Int31()
	genCallInput := 21.3
	var resp *Response

	var wg sync.WaitGroup
	wg.Add(1)
	errChan := make(chan error, 2)

	var sharedErr error

	safego.Go(func() {
		defer wg.Done()
		_, err := generalFCall(ctx, genCallInput)
		if err != nil {
			errChan <- err
		}
		_, err1 := generalNilECall(ctx, genCallInput)
		if err1 != nil {
			errChan <- err1
		}
		_, err3 := notContextCall(genCallInput)
		sharedErr = err3
	})

	wg.Wait()
	close(errChan)
	for err := range errChan {
		return nil, err
	}
	fmt.Println(sharedErr)

	return resp, nil
}

// BFunctionWithUniqueName should not trigger safego workflow.
func BFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	//random := rand.Int31()
	var resp *Response
	var sharedErr error
	fmt.Println(sharedErr)
	return resp, nil
}

// CFunctionWithUniqueName has safego but no error back
func CFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	//random := rand.Int31()
	genCallInput := 21.3
	var resp *Response

	var wg sync.WaitGroup
	wg.Add(1)

	safego.Go(func() {
		defer wg.Done()
		_, err := generalFCall(ctx, genCallInput)
		if err != nil {
			fmt.Println(err)
		}
	})

	wg.Wait()
	return resp, nil
}

// DFunctionWithUniqueName has safego but no qualified binding
func DFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	genCallInput := 21.3
	var resp *Response

	var wg sync.WaitGroup
	wg.Add(1)

	safego.Go(func() {
		defer wg.Done()
		_, err := notContextCall(genCallInput)
		if err != nil {
			fmt.Println(err)
		}
	})

	wg.Wait()
	return resp, nil
}

// EFunctionWithUniqueName has safego and anonymous function pass as functor
func EFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	genCallInput := 21.3
	var resp *Response
	var sharedErr error

	var wg sync.WaitGroup
	wg.Add(1)
	errChan := make(chan error, 1)
	fnlamd := func() {
		defer wg.Done()
		_, err := generalNilECall(ctx, genCallInput)
		if err != nil {
			fmt.Println("Error:", err)
			errChan <- err
		}
		_, err1 := generalFCall(ctx, genCallInput)
		if err1 != nil {
			sharedErr = err1
		}
	}

	safego.Go(fnlamd)
	wg.Wait()
	close(errChan)
	fmt.Println(sharedErr)
	for err := range errChan {
		return nil, err
	}

	return resp, nil
}

// GFunctionWithUniqueName has safego and func rewrite
func GFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	genCallInput := 21.3
	var resp *Response

	var wg sync.WaitGroup
	wg.Add(1)
	errChan := make(chan error, 1)
	fnlamd := func() {
		defer wg.Done()
		_, err := generalNilECall(ctx, genCallInput)
		if err != nil {
			fmt.Println("Error:", err)
			errChan <- err
		}
	}
	// functor rewrite
	tt := fnlamd

	safego.Go(tt)
	wg.Wait()
	close(errChan)
	for err := range errChan {
		return nil, err
	}

	return resp, nil
}

// HFunctionWithUniqueName has safego and anonymous function pass as functor
func HFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	genCallInput := 21.3
	var resp *Response
	var sharedErr error

	var wg sync.WaitGroup
	wg.Add(1)
	errChan := make(chan error, 1)
	fnlamd := func() {
		defer wg.Done()
		_, err := generalNilECall(ctx, genCallInput)
		if err != nil {
			errChan <- err
		}
		_, err2 := generalFCall(ctx, genCallInput)
		if err2 != nil {
			fmt.Println("Error:", err2)
		}
		_, err1 := notContextCall(genCallInput)
		if err1 != nil {
			sharedErr = err1
		}
	}

	safego.Go(fnlamd)
	wg.Wait()
	close(errChan)
	fmt.Println(sharedErr)
	for err := range errChan {
		return nil, err
	}

	return resp, nil
}

// IFunctionWithUniqueName has safego and func rewrite
func IFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	genCallInput := 21.3
	var resp *Response

	var wg sync.WaitGroup
	wg.Add(1)
	errChan := make(chan error, 1)

	safego.Go(func() {
		defer wg.Done()
		randomInt := rand.Intn(2) // Generates either 0 or 1
		flipdoor(randomInt == 0)  // Flip the door
		_, err := crazyF(ctx, genCallInput)
		if err != nil {
			errChan <- err
		}
	})
	wg.Wait()
	close(errChan)
	for err := range errChan {
		return nil, err
	}

	return resp, nil
}

// JFunctionWithUniqueName test M:N mapping
func JFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	//random := rand.Int31()
	genCallInput := 21.3
	var resp *Response

	var wg sync.WaitGroup
	wg.Add(1)
	errChan := make(chan error, 1)
	errChan1 := make(chan error, 2)

	safego.Go(func() {
		defer wg.Done()
		_, err := generalFCall(ctx, genCallInput)
		if err != nil {
			errChan <- err
		}
		_, err1 := generalFCall(ctx, genCallInput)
		if err1 != nil {
			errChan1 <- err1
		}
		_, err2 := generalTCall(ctx, genCallInput)
		if err2 != nil {
			errChan1 <- err2
		}
	})

	wg.Wait()
	close(errChan)
	close(errChan1)
	for err := range errChan {
		return nil, err
	}
	for err := range errChan1 {
		return nil, err
	}
	return resp, nil
}

// KFunctionWithUniqueName is pass synamic fn to safego
func KFunctionWithUniqueName() (*Response, error) {
	var resp *Response
	var wg sync.WaitGroup
	wg.Add(1)

	safego.Go(staticFunc)

	wg.Wait()
	for err := range globalErrChan {
		return nil, err
	}
	return resp, nil
}

func main() {
	globalErrChan = make(chan error, 1)
	ctx := context.Background()
	resp, err := AFunctionWithUniqueName(ctx, "lucky")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Response:", resp)

	// This will not be caught by the safego enricher
	resp, err = BFunctionWithUniqueName(ctx, "nolucky")

	resp, err = CFunctionWithUniqueName(ctx, "nolucky")

	resp, err = DFunctionWithUniqueName(ctx, "nolucky")

	resp, err = EFunctionWithUniqueName(ctx, "lucky")

	resp, err = GFunctionWithUniqueName(ctx, "lucky")

	resp, err = HFunctionWithUniqueName(ctx, "lucky")

	resp, err = IFunctionWithUniqueName(ctx, "lucky")

	resp, err = JFunctionWithUniqueName(ctx, "lucky")

	resp, err = KFunctionWithUniqueName()
}
