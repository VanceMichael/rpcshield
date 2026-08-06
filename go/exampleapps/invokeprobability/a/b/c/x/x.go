package x

import "context"

// Controller is an interface
type Controller interface {
	F1(context.Context, int) error
	F2(context.Context, int) error
}

type controllerImpl struct {
}

// NewController is a function.
func NewController() Controller {
	return &controllerImpl{}
}

// F1 is a method
func (c *controllerImpl) F1(ctx context.Context, i int) error {
	return nil
}

// F2 is a method
func (c *controllerImpl) F2(ctx context.Context, i int) error {
	return nil
}

// OtherName is an interface
type OtherName interface {
	F1(context.Context, int) error
	F2(context.Context, int) error
}

// NewOtherName is a function.
func NewOtherName() OtherName {
	return &otherNameImpl{}
}

type otherNameImpl struct {
}

// F1 is a method
func (c *otherNameImpl) F1(ctx context.Context, i int) error {
	return nil
}

// F2 is a method
func (c *otherNameImpl) F2(ctx context.Context, i int) error {
	return nil
}
