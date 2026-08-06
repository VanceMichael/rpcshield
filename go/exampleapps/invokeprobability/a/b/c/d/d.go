package d

import "context"

// Controller is an interface
type Controller interface {
	F1(context.Context, int) error
	F2(context.Context, int) error
}

// NewController is a function.
func NewController() Controller {
	return &controllerImpl{}
}

type controllerImpl struct {
}

// F1 is a method
func (c *controllerImpl) F1(ctx context.Context, i int) error {
	return nil
}

// F2 is a method
func (c *controllerImpl) F2(ctx context.Context, i int) error {
	return nil
}
