// Package thrifthandler implements the demo's second inbound transport
// (yarpc's Thrift encoding), analogous to cff-demo's TChannel handler.
package thrifthandler

import (
	"context"
	"errors"

	"thriftrw/simpleservice"
	"thriftrw/simpleservice/simpleserviceclient"
	"thriftrw/simpleservice/simpleserviceserver"

	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller"
)

type handler struct {
	ctrl   controller.Controller
	client simpleserviceclient.Interface
}

// New returns a new simpleserviceserver.Interface implementation.
func New(ctrl controller.Controller, client simpleserviceclient.Interface) simpleserviceserver.Interface {
	return &handler{ctrl: ctrl, client: client}
}

// Create implements simpleserviceserver.Interface.
func (h *handler) Create(ctx context.Context, msg *simpleservice.Message) (*simpleservice.Message, error) {
	res, err := h.ctrl.Create(ctx, msg.Content)
	if err != nil {
		return nil, err
	}

	// Call the service itself to test outbound support through this transport.
	if msg.Content == "recursive" {
		_, err := h.client.Create(ctx, msg)
		if err != nil {
			return nil, err
		}
	}

	if res != nil {
		return &simpleservice.Message{Content: res.Name}, nil
	}
	return nil, errors.New("failed to create message")
}
