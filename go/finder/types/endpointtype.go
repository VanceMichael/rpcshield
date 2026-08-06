package types

// EndpointType is a shared transport type used by inbound and outbound points.
type EndpointType string

const (
	// TypeGRPC is the type of the call that uses gRPC.
	TypeGRPC EndpointType = "grpc"
	// TypeThrift is the type of the call that uses Thrift.
	TypeThrift EndpointType = "thrift"
	// TypeThriftTChannel is the type of the call that uses Thrift over TChannel.
	TypeThriftTChannel EndpointType = "tchannel"
	// TypeHTTP is the type of the call that uses HTTP.
	TypeHTTP EndpointType = "http"
	// TypeCadence is the type of the call that uses Cadence.
	TypeCadence EndpointType = "cadence"
)
