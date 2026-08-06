// Package nopservice provides a stub implementation of RpcshieldDemoYARPCServer.
// This is used to test the inbound detection fix in rpcshield that correctly
// identifies handlers where the value type embeds a nopservice with pointer
// receivers, meaning only *handler implements the interface.
package nopservice

import (
	"context"
	"errors"

	"gogoproto/rpcshielddemo"
)

// Server is a stub implementation of RpcshieldDemoYARPCServer.
// All methods use pointer receivers, so only *Server implements the interface.
type Server struct{}

var _ rpcshielddemo.RpcshieldDemoYARPCServer = (*Server)(nil)

// Create returns unimplemented.
func (s *Server) Create(context.Context, *rpcshielddemo.CreateRequest) (*rpcshielddemo.CreateResponse, error) {
	return nil, errors.New("unimplemented")
}

// Get returns unimplemented.
func (s *Server) Get(context.Context, *rpcshielddemo.GetRequest) (*rpcshielddemo.GetResponse, error) {
	return nil, errors.New("unimplemented")
}
