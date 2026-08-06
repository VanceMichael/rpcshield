// Package handler implements the rpcshielddemo service's gRPC/YARPC handlers.
package handler

import (
	"context"

	"gogoproto/rpcshielddemo"

	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller"
)

// Handler represents the rpcshielddemo service's gRPC API.
type Handler = rpcshielddemo.RpcshieldDemoYARPCServer

type handler struct {
	ctrl controller.Controller
}

// New constructs a new rpcshielddemo Handler.
func New(ctrl controller.Controller) Handler {
	return &handler{ctrl: ctrl}
}

// Create synchronously creates and saves a new Result.
func (h *handler) Create(ctx context.Context, r *rpcshielddemo.CreateRequest) (*rpcshielddemo.CreateResponse, error) {
	res, err := h.ctrl.Create(ctx, r.Name)
	if err != nil {
		return nil, err
	}
	return &rpcshielddemo.CreateResponse{Uuid: res.Name}, nil
}

// Get synchronously retrieves a previously-created Result.
func (h *handler) Get(ctx context.Context, r *rpcshielddemo.GetRequest) (*rpcshielddemo.GetResponse, error) {
	res, err := h.ctrl.Get(ctx, r.Uuid)
	if err != nil {
		return nil, err
	}
	return &rpcshielddemo.GetResponse{Name: res.Name}, nil
}
