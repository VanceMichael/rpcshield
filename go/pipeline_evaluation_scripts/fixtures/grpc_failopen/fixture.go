// Command fixture is a known-answer test overlaid into a real OSS module (see
// ../../README.md's "Known-answer fixture" section) to validate that rpcshield's generic
// gRPC/Thrift detection produces *correct* fail-open/fail-close classifications against real
// generated code, not just that the pipeline runs without crashing. It reuses etcd's real
// etcdserverpb-generated KV client/server types (google.golang.org/grpc-based) rather than
// inventing new ones. It is merged into the target module via packages.Config.Overlay (see
// run_oss_pipeline_sweep.sh -fixture-file), not built/run for real -- package main only so its
// main() gives RTA a real root to reach both directions from (the target module otherwise has
// none, see README's "Gotchas" section).
//
// etcdserverpb (not yarpc-go) is the type source deliberately: yarpc-go's own package path
// (go.uber.org/yarpc/...) is one of graph.DefaultExcludePackagePatterns's entries -- rpcshield
// treats YARPC as generic framework plumbing to exclude from call-graph construction, which is
// correct when YARPC is a dependency of the service under analysis but silently drops the entire
// fixture and its generated types from the call graph when YARPC's own code *is* the analyzed
// target. etcd's module path has no such collision.
//
// Each inbound handler below makes its own downstream call (rather than delegating to a separate,
// call-graph-disconnected helper function) -- this is both the structurally-realistic shape of a
// fail-open/fail-close bug (an inbound handler propagating or swallowing a downstream error) and
// load-bearing for RTA: rpcshield's RTA implementation only creates a callgraph.Node for a
// function when it appears as a caller or callee of some edge (see rta/srta/impl.go), so an
// inbound "root" function with no outgoing calls of its own is never found in the built call
// graph (see graph/cct.go's buildCallTreeNode) even though it was passed to RTA as a root.
package main

import (
	"context"
	"net"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc"
)

// kvServerFailOpen implements etcdserverpb.KVServer, the shape findGenericGRPCServer should
// detect via the etcdserverpb.RegisterKVServer call in serveFailOpen. Its Put method calls a
// downstream Put client (the shape isGenericGRPCClient/genericGRPCThriftOutbound should detect)
// and swallows any error -- expected classification: fail_open.
type kvServerFailOpen struct {
	etcdserverpb.UnimplementedKVServer
	client etcdserverpb.KVClient
}

func (s kvServerFailOpen) Put(ctx context.Context, req *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
	resp, err := s.client.Put(ctx, req)
	if err != nil {
		return &etcdserverpb.PutResponse{}, nil
	}
	return resp, nil
}

// kvServerFailClose implements etcdserverpb.KVServer the same way, but its Put method propagates
// the downstream error instead of swallowing it -- expected classification: fail_close.
type kvServerFailClose struct {
	etcdserverpb.UnimplementedKVServer
	client etcdserverpb.KVClient
}

func (s kvServerFailClose) Put(ctx context.Context, req *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
	resp, err := s.client.Put(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// serveFailOpen registers kvServerFailOpen on a real grpc.Server, the inbound call site
// findGenericGRPCServer should detect.
func serveFailOpen(lis net.Listener, client etcdserverpb.KVClient) error {
	s := grpc.NewServer()
	etcdserverpb.RegisterKVServer(s, &kvServerFailOpen{client: client})
	return s.Serve(lis)
}

// serveFailClose registers kvServerFailClose on a separate real grpc.Server, a second inbound
// call site findGenericGRPCServer should detect.
func serveFailClose(lis net.Listener, client etcdserverpb.KVClient) error {
	s := grpc.NewServer()
	etcdserverpb.RegisterKVServer(s, &kvServerFailClose{client: client})
	return s.Serve(lis)
}

func main() {
	lis1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	lis2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	cc, err := grpc.Dial(lis1.Addr().String(), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	client := etcdserverpb.NewKVClient(cc)

	go serveFailOpen(lis1, client)
	go serveFailClose(lis2, client)
}
