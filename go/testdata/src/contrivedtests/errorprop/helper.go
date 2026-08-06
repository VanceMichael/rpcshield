package errorprop

import (
	"errors"
	"fmt"
)

var cond, cond2, cond3 bool

var (
	GlobalErrTrusted                         error = errors.New("global error")
	GlobalErrNewErrorPattern                       = NewValidationError("some_err_type")
	_globalErrNil                            error = nil
	GlobalErrMultiDecl1, GlobalErrMultiDecl2 error = errors.New("multi1"), errors.New("multi2")
	GlobalErrUninitialized                   error
	GlobalErrWrapped                                      = wrapError(fmt.Errorf("global error"))
	GlobalErrInterface                       ErrorWrapper = NewInternalErrorWrapper(errors.New("interface"))
)

type myErr struct{}

func (myErr) Error() string { return "myErr message" }

type ErrorWrapper interface {
	error
	LogErr(string)
}

type InternalErrorWrapper struct {
	sourceErr error
}

func NewInternalErrorWrapper(err error) ErrorWrapper {
	return &InternalErrorWrapper{sourceErr: err}
}

func (i *InternalErrorWrapper) Error() string {
	return "internal error"
}

func (i *InternalErrorWrapper) LogErr(string) {}

type InternalErrorWrapper2 struct {
	sourceErr error
}

func (i InternalErrorWrapper2) Error() string {
	return "internal error"
}

type S struct {
	f *int
}

func OutboundRPC() (*S, error) {
	if cond {
		return nil, errors.New("error")
	}
	return &S{}, nil
}

func OutboundRPCRetEmbeddedErr() (*S, ErrorWrapper) {
	if cond {
		return nil, NewInternalErrorWrapper(errors.New("error"))
	}
	return &S{}, nil
}

func nonOutboundRetPtrError() (*S, error) {
	if cond {
		return nil, errors.New("error")
	}
	return &S{}, nil
}

func logError(err error) {}

func retAnotherError() error {
	return &myErr{}
}

func wrapError(err error) error {
	return fmt.Errorf("error %w", err)
}

func wrapErrorManyArgsOneRet(ctx string, msg string, err error, id int) error {
	return fmt.Errorf("error %w for %s %s %d", err, ctx, msg, id)
}

func wrapErrorOneArgsManyRet(err error) (*int, error, *string) {
	return nil, fmt.Errorf("error %w", err), new(string)
}

func wrapErrorManyArgsManyRet(ctx string, msg string, err error, id int) (*int, error, *string) {
	return &id, fmt.Errorf("error %w for %s %s %d", err, ctx, msg, id), &msg
}

type I interface {
	Foo() (*S, error)
}

func (s *S) Foo() (*S, error) {
	return OutboundRPC()
}

func retErr() error {
	if cond {
		return errors.New("error")
	}
	return nil
}

func NewError(msg string) error {
	return errors.New(msg)
}

func NewInternalError(msg string) error {
	return fmt.Errorf(msg)
}

func NewValidationError(t string) ValidationError {
	return ValidationError{Type: t}
}

type ValidationError struct {
	Type string
}

// Error returns a ValidationError message.
func (err ValidationError) Error() string { return err.Type }
