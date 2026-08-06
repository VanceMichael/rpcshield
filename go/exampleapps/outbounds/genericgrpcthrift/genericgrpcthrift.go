// Command genericgrpcthrift is a fixture that calls into package pb (a stand-in for
// protoc-gen-go-grpc/Apache-Thrift-generated code) the way real generated-code callers do:
// package-qualified for server registration, via a local client value for outbound calls. See
// pb's doc comment for why the generated shapes live in a separate package.
package main

import (
	"context"

	"google.golang.org/grpc"

	"github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift/pb"
)

// callFoo exercises pb.FooClient.Foo as an outbound call site.
func callFoo(c *pb.FooClient) error {
	return c.Foo(context.Background())
}

// fooServerImpl implements pb.FooServer, the shape findGenericGRPCServer should detect via the
// pb.RegisterFooServer call below.
type fooServerImpl struct{}

func (fooServerImpl) Foo(context.Context) error { return nil }

// setupGRPCServer registers fooServerImpl, the call site findGenericGRPCServer should detect.
func setupGRPCServer() {
	s := grpc.NewServer()
	pb.RegisterFooServer(s, fooServerImpl{})
}

// callBar exercises pb.BarClient.Bar as an outbound call site.
func callBar(c *pb.BarClient) error {
	return c.Bar(context.Background())
}

// barHandlerImpl implements pb.BarHandler, the shape findGenericThriftServer should detect via
// the pb.NewBarProcessor call below.
type barHandlerImpl struct{}

func (barHandlerImpl) Bar(context.Context) error { return nil }

// setupThriftServer constructs a processor for barHandlerImpl, the call site
// findGenericThriftServer should detect.
func setupThriftServer() {
	pb.NewBarProcessor(barHandlerImpl{})
}

func main() {}
