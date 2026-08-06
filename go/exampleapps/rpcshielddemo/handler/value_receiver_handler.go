package handler

import (
	"context"

	"gogoproto/rpcshielddemo"

	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller"
	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/pkg/nopservice"
)

// valueReceiverHandler embeds nopservice.Server (which has pointer receiver
// methods) and overrides Create with a value receiver. This means only
// *valueReceiverHandler implements rpcshielddemo.RpcshieldDemoYARPCServer, not
// valueReceiverHandler by itself. This tests finder's ability to check both T
// and *T when detecting handlers.
type valueReceiverHandler struct {
	nopservice.Server
	ctrl controller.Controller
}

// NewValueReceiverHandler constructs a handler for testing the pointer-type
// detection fix.
func NewValueReceiverHandler(ctrl controller.Controller) Handler {
	return &valueReceiverHandler{ctrl: ctrl}
}

// Create uses a value receiver. Combined with the embedded nopservice's
// pointer-receiver Get, only *valueReceiverHandler implements
// RpcshieldDemoYARPCServer.
func (h valueReceiverHandler) Create(ctx context.Context, r *rpcshielddemo.CreateRequest) (*rpcshielddemo.CreateResponse, error) {
	res, err := h.ctrl.Create(ctx, r.Name)
	if err != nil {
		return nil, err
	}
	return &rpcshielddemo.CreateResponse{Uuid: res.Name}, nil
}
