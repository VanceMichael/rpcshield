// This is a standalone module, deliberately excluded from the parent rpcshield module's own
// build/lint (see ../../README.md's "Known-answer fixture" section and .golangci.yml at the repo
// root). fixture.go's *content* is read verbatim (os.ReadFile, not `go build`) and overlaid into a
// real OSS target module (e.g. etcd) by cmd/oss_pipeline_bench, so it never actually compiles
// against these requirements -- this go.mod exists only so `go build ./...`/golangci-lint at the
// repo root don't fail trying to resolve etcdserverpb/grpc as part of the parent module.
module github.com/uber-research/rpcshield/go/pipeline_evaluation_scripts/fixtures

go 1.26.1

require (
	go.etcd.io/etcd/api/v3 v3.7.1
	google.golang.org/grpc v1.82.1
)

require (
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
