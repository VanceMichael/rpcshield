# Contributing

## Build and test

The primary, CI-verified build is Bazel:

```bash
bazel build //...
bazel test //...
```

After adding or changing imports, regenerate `BUILD.bazel` files before building:

```bash
bazel run //:gazelle
```

Plain `go build ./...`/`go vet ./...` also work for everything except four packages (the root
package, `output/`, `genai/`, `common/utils/`) whose test files import Bazel-generated gomock
mocks via an importmap path that only resolves inside Bazel — see `.golangci.yml`'s comment for
details.

## Linting and formatting

This repo uses [golangci-lint](https://golangci-lint.run/) (config: `.golangci.yml`), which bundles
`gofmt`/`goimports` formatting checks together with `govet`, `staticcheck`, `unused`,
`ineffassign`, `misspell`, `unconvert`, and `errcheck`.

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6
golangci-lint run ./...
```

CI (`.github/workflows/lint.yml`) only fails a pull request on issues introduced by that PR's own
diff (`--new-from-rev`) — pre-existing findings elsewhere in the codebase don't block your change,
but please don't add new ones.

## Validating detection changes against real services

If you're changing anything under `finder/`, `graph/`, or `staticanalyzer/`, validate the change
against real open-source Go services (not just unit tests) using
[`pipeline_evaluation_scripts/`](pipeline_evaluation_scripts/README.md) — a harness that runs the
full pipeline against real OSS repos (etcd, temporal, thanos, vitess, jaeger, and others) plus a
known-answer fixture that checks for *correct* fail-open/fail-close classification, not just a
crash-free run.
