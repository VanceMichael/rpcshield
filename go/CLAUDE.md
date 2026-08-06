# CLAUDE.md

Guidance for working in this repo (`github.com/uber-research/rpcshield/go`), the standalone OSS
fork of Uber's internal RPCShield whole-program fail-open/fail-close analyzer. See `README.md` for
architecture; this file is about how to build/test/verify changes correctly.

## Build & test: use Bazel, not plain `go test`

```bash
bazel build //...
bazel test //...
```

Scope to a package during iteration, e.g. `bazel test //finder/servicemapping/... //graph/...`.

`go build ./...` and `go vet ./...` work fine as a quick sanity check. **`go test` does not** —
several packages (e.g. the root `driver_test.go`, `output/output_test.go`) import mocks generated
by Bazel's gomock rule into a GOPATH-style `mock/github.com/...` import path that only exists inside
Bazel's sandbox/runfiles tree. Running plain `go test` fails with `package mock/... is not in std`
even when nothing is actually broken — this looks like a real failure but isn't one. Always verify
with `bazel test`, and don't conclude a package is broken based on a plain `go test` failure without
first confirming the same failure exists under `bazel test`.

## Beyond unit tests: the OSS sweeps

Unit tests exercise individual packages against small fixtures. Two standalone harnesses drive the
*real* pipeline end-to-end against real open-source Go services, and are the way to catch
regressions in core detection/call-graph logic that unit tests won't:

- **`pipeline_evaluation_scripts/`** — runs the full fail-close/fail-open pipeline (finder, RTA,
  SSA/SCCP analyzer, reporting) against OSS services using standard gRPC/Thrift (etcd, temporal,
  thanos, vitess, cortex, jaeger, etc. — see `targets.csv`). Run `./check_fixture.sh` first (a
  known-answer correctness check), then `./run_oss_pipeline_sweep.sh -output <dir>` for the full
  sweep; compare the per-target inbound/outbound/report counts against `README.md`'s "Current sweep
  status" table to catch regressions.
- **`rta/evaluation_scripts/`** — runs just the RTA call-graph construction (all flavors) against
  the same kind of OSS targets; use `./run_oss_rta_sweep.sh` per its `README.md`.

Run the relevant sweep (or both, if the change is central to call-graph/detection logic) before
considering a change to `finder/`, `graph/`, `rta/`, `staticanalyzer/`, or `driver.go` complete —
`bazel test` passing is necessary but not sufficient for that kind of change.

## Code comments

Comments should describe the code's current architecture and behavior, not the process that
produced it. Avoid narrating a migration/porting/extraction history in comments (e.g. "so it can be
migrated to X", "starts empty and grows one field per package migration") — write for the code's
final shape, as if it always looked this way. TODOs and documented known issues are the only
migration-adjacent comments that belong in the code.
