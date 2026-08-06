#!/usr/bin/env bash
# Runs oss_pipeline_bench against the known-answer fixture (fixtures/grpc_failopen/fixture.go,
# overlaid into a real dataset via -fixture-file) and asserts the pipeline produces the *correct*
# fail-open/fail-close classification for both known call paths -- not just that it runs without
# crashing. See ../README.md's "Known-answer fixture" section.
#
# Usage:
#   ./check_fixture.sh [-binary PATH] [-datasets DIR]
#
# Exits 0 and prints PASS if both expected classifications are found in the output CSV; exits 1
# and prints FAIL with a diagnostic otherwise.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

_real_goroot="$(go env GOROOT 2>/dev/null || true)"
if [[ -x "${_real_goroot}/bin/go" ]]; then
  export PATH="${_real_goroot}/bin:${PATH}"
  export GOROOT="${_real_goroot}"
fi
export GO111MODULE=on
unset GOFLAGS GOPACKAGESDRIVER GOPACKAGESDRIVER_ULSP_MODE 2>/dev/null || true

DATASETS_DIR="${SCRIPT_DIR}/datasets"
BINARY=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -binary)   BINARY="${2:?}";   shift 2;;
    -datasets) DATASETS_DIR="${2:?}"; shift 2;;
    -h|--help) sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'; exit 0;;
    *) echo "unknown arg: $1" >&2; exit 1;;
  esac
done

WORKSPACE="$(git rev-parse --show-toplevel 2>/dev/null || true)"
RPCSHIELD_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BAZEL_BIN_REL="pipeline_evaluation_scripts/cmd/oss_pipeline_bench/oss_pipeline_bench_/oss_pipeline_bench"

if [[ -z "${BINARY}" ]]; then
  if [[ -x "${RPCSHIELD_ROOT}/bazel-bin/${BAZEL_BIN_REL}" ]]; then
    BINARY="${RPCSHIELD_ROOT}/bazel-bin/${BAZEL_BIN_REL}"
  else
    echo "error: no oss_pipeline_bench binary found; build it first:" >&2
    echo "  bazel build //pipeline_evaluation_scripts/cmd/oss_pipeline_bench:oss_pipeline_bench" >&2
    echo "or pass -binary PATH" >&2
    exit 1
  fi
fi

FIXTURE_FILE="${SCRIPT_DIR}/fixtures/grpc_failopen/fixture.go"
OUTPUT_DIR="$(mktemp -d)"
trap 'rm -rf "${OUTPUT_DIR}"' EXIT

echo "[run] ${BINARY} against ${FIXTURE_FILE}"
rc=0
"${BINARY}" \
  -dataset-root="${DATASETS_DIR}" \
  -target-name=fixture-check \
  -target-dir=etcd/api \
  -target-pkg=./... \
  -codebase-pkg-prefix=go.etcd.io/etcd \
  -fixture-file="${FIXTURE_FILE}" \
  -output-dir="${OUTPUT_DIR}" \
  >"${OUTPUT_DIR}/log.txt" 2>&1 || rc=$?

CSV="${OUTPUT_DIR}/fixture-check.csv"
if [[ $rc -ne 0 || ! -s "${CSV}" ]]; then
  echo "FAIL: oss_pipeline_bench did not produce a CSV report (exit=$rc)"
  echo "--- log tail ---"
  tail -n 60 "${OUTPUT_DIR}/log.txt"
  exit 1
fi

fail=0
if grep -q 'kvServerFailOpen).Put.*FailOpen\|FailOpen.*kvServerFailOpen).Put' "${CSV}"; then
  echo "  [ok] kvServerFailOpen.Put classified FailOpen"
else
  echo "  [MISSING] expected kvServerFailOpen.Put -> FailOpen not found in CSV"
  fail=1
fi
if grep -q 'kvServerFailClose).Put.*FailClose\|FailClose.*kvServerFailClose).Put' "${CSV}"; then
  echo "  [ok] kvServerFailClose.Put classified FailClose"
else
  echo "  [MISSING] expected kvServerFailClose.Put -> FailClose not found in CSV"
  fail=1
fi

if [[ "${fail}" -eq 0 ]]; then
  echo "PASS"
  exit 0
fi
echo "FAIL: see CSV at ${CSV} (copy preserved below since OUTPUT_DIR is a tempdir)"
cp "${CSV}" "/tmp/fixture-check-failed-$(date +%s).csv"
exit 1
