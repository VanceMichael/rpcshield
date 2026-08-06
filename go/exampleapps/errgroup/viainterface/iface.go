package viainterface

// VIAInterface is a simple interface that mimics the behavior of the errgroup package.
type VIAInterface interface {
	Go(fun func() error)
	Wait() error
}

type viaIfaceImpl struct{}

// GetVIAInterface returns a new instance of the VIAInterface.
func GetVIAInterface() VIAInterface {
	return &viaIfaceImpl{}
}

// Go is a simple function that calls the provided function.
func (impl *viaIfaceImpl) Go(fun func() error) {
	fun()
}

// Wait is a simple function that returns nil.
func (impl *viaIfaceImpl) Wait() error {
	return nil
}
