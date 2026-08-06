#!/usr/bin/env bash
# Runs oss_pipeline_bench (rpcshield's real fail-close/fail-open analysis, not just RTA
# construction) across all targets in a manifest, writing one ${OUTPUT_DIR}/${target}/ directory
# (driver log + CSV/HTML reports) per target. Structural fork of
# ../rta/evaluation_scripts/run_oss_rta_sweep.sh adapted for the full pipeline.
#
# Usage:
#   ./run_oss_pipeline_sweep.sh [options]
#
# Options:
#   -targets FILE   Path to targets manifest. Default: ./targets.csv
#                   CSV with header row; columns: name,repo,tag,dir,pkg,service_name,codebase_pkg_prefix
#   -datasets DIR   Root containing cloned OSS repos. Default: ./datasets
#                   (the same DATASETS_ROOT that clone_datasets.sh used)
#   -output DIR     Output directory; each target gets its own ${OUTPUT_DIR}/${name}/ subdir.
#                   Default: /tmp/oss_pipeline_stats_$(date +%Y%m%d_%H%M%S)
#   -timeout SEC    Per-target timeout (passed to `timeout` cmd). Default: 5400
#   -binary PATH    Pre-built oss_pipeline_bench binary. If unset, the script tries:
#                     1. ./oss_pipeline_bench next to this script
#                     2. bazel-bin/.../oss_pipeline_bench
#                     3. `bazel build` of the target (cached after first run)
#   -force          Re-run targets even if their log already shows a completion marker.
#   -dry-run        Print the commands that would be run, then exit.
#   -h | --help     Show this help.
#
# Env vars:
#   RPCSHIELD_REPO_REL_PATH  Path of the rpcshield repo root relative to the Bazel workspace
#                            root, used for the "bazel build" binary-resolution fallback.
#                            Default: derived from this script's own location (this script lives
#                            at rpcshield/pipeline_evaluation_scripts/, one directory below the
#                            rpcshield repo root, wherever that repo is checked out). Override
#                            for unusual layouts (e.g. a symlinked checkout) where that
#                            derivation doesn't resolve.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Some monorepos' $PATH `go` is a wrapper that hardcodes GO111MODULE=off and sources a
# per-project env.sh, which breaks `go list` against OSS checkouts. Switch to the real go binary
# in GOROOT and force module mode for everything we and the harness binary's go/packages driver
# invoke. (Same workaround as rta/evaluation_scripts/run_oss_rta_sweep.sh.)
_real_goroot="$(go env GOROOT 2>/dev/null || true)"
if [[ -x "${_real_goroot}/bin/go" ]]; then
  export PATH="${_real_goroot}/bin:${PATH}"
  export GOROOT="${_real_goroot}"
fi
export GO111MODULE=on
unset GOFLAGS GOPACKAGESDRIVER GOPACKAGESDRIVER_ULSP_MODE 2>/dev/null || true

TARGETS_FILE="${SCRIPT_DIR}/targets.csv"
DATASETS_DIR="${SCRIPT_DIR}/datasets"
OUTPUT_DIR=""
TIMEOUT=5400
BINARY=""
FORCE=0
DRY_RUN=0

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -targets)  TARGETS_FILE="${2:?}"; shift 2;;
    -datasets) DATASETS_DIR="${2:?}"; shift 2;;
    -output)   OUTPUT_DIR="${2:?}";   shift 2;;
    -timeout)  TIMEOUT="${2:?}";      shift 2;;
    -binary)   BINARY="${2:?}";       shift 2;;
    -force)    FORCE=1;               shift;;
    -dry-run)  DRY_RUN=1;             shift;;
    -h|--help) usage 0;;
    *) echo "unknown arg: $1" >&2; usage 1;;
  esac
done

if [[ ! -r "${TARGETS_FILE}" ]]; then
  echo "error: targets file not readable: ${TARGETS_FILE}" >&2
  exit 1
fi

DATASETS_DIR="$(cd "${DATASETS_DIR}" 2>/dev/null && pwd || echo "${DATASETS_DIR}")"

if [[ -z "${OUTPUT_DIR}" ]]; then
  OUTPUT_DIR="/tmp/oss_pipeline_stats_$(date +%Y%m%d_%H%M%S)"
fi
mkdir -p "${OUTPUT_DIR}"

WORKSPACE="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -n "${WORKSPACE}" ]]; then
  RPCSHIELD_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
  RPCSHIELD_REPO_REL_PATH="${RPCSHIELD_REPO_REL_PATH:-$(realpath --relative-to="${WORKSPACE}" "${RPCSHIELD_ROOT}")}"
fi
RPCSHIELD_REPO_REL_SEGMENT="${RPCSHIELD_REPO_REL_PATH:-}"
[[ "${RPCSHIELD_REPO_REL_SEGMENT}" == "." ]] && RPCSHIELD_REPO_REL_SEGMENT=""
[[ -n "${RPCSHIELD_REPO_REL_SEGMENT}" ]] && RPCSHIELD_REPO_REL_SEGMENT="${RPCSHIELD_REPO_REL_SEGMENT}/"
BAZEL_TARGET="//${RPCSHIELD_REPO_REL_SEGMENT}pipeline_evaluation_scripts/cmd/oss_pipeline_bench:oss_pipeline_bench"
BAZEL_BIN_REL="bazel-bin/${RPCSHIELD_REPO_REL_SEGMENT}pipeline_evaluation_scripts/cmd/oss_pipeline_bench/oss_pipeline_bench_/oss_pipeline_bench"

resolve_binary() {
  if [[ -n "${BINARY}" ]]; then
    [[ -x "${BINARY}" ]] || { echo "error: -binary not executable: ${BINARY}" >&2; exit 1; }
    return
  fi
  if [[ -x "${SCRIPT_DIR}/oss_pipeline_bench" ]]; then
    BINARY="${SCRIPT_DIR}/oss_pipeline_bench"; return
  fi
  if [[ -n "${WORKSPACE}" && -x "${WORKSPACE}/${BAZEL_BIN_REL}" ]]; then
    BINARY="${WORKSPACE}/${BAZEL_BIN_REL}"; return
  fi
  if [[ -n "${WORKSPACE}" ]]; then
    echo "[build] bazel build ${BAZEL_TARGET}"
    if [[ "${DRY_RUN}" -eq 0 ]]; then
      (cd "${WORKSPACE}" && bazel build "${BAZEL_TARGET}") >&2
      BINARY="${WORKSPACE}/${BAZEL_BIN_REL}"
    else
      BINARY="${WORKSPACE}/${BAZEL_BIN_REL}"
    fi
    return
  fi
  echo "error: could not locate oss_pipeline_bench binary; pass -binary PATH or run from a git checkout" >&2
  exit 1
}
resolve_binary

# Read CSV manifest into parallel arrays (skip header row).
NAMES=(); REPOS=(); TAGS=(); DIRS=(); PKGS=(); SERVICES=(); PREFIXES=(); DROPS=()
{
  read -r _header  # discard the first line
  while IFS=, read -r name repo tag dir pkg service_name codebase_pkg_prefix drop_patterns; do
    [[ -z "${name}" ]] && continue
    if [[ -z "${repo}" || -z "${tag}" || -z "${dir}" || -z "${pkg}" || -z "${codebase_pkg_prefix}" ]]; then
      echo "[warn] malformed row, skipping: name=${name} repo=${repo} tag=${tag} dir=${dir} pkg=${pkg} codebase_pkg_prefix=${codebase_pkg_prefix}" >&2
      continue
    fi
    NAMES+=("${name}"); REPOS+=("${repo}"); TAGS+=("${tag}"); DIRS+=("${dir}"); PKGS+=("${pkg}")
    SERVICES+=("${service_name:-${name}}"); PREFIXES+=("${codebase_pkg_prefix}")
    # drop_patterns is optional and semicolon-separated in the CSV (commas are the CSV field
    # delimiter); oss_pipeline_bench's -drop-exclude-package-patterns flag wants comma-separated,
    # so translate here.
    DROPS+=("${drop_patterns//;/,}")
  done
} < "${TARGETS_FILE}"

TOTAL=${#NAMES[@]}
if [[ "${TOTAL}" -eq 0 ]]; then
  echo "error: no targets found in ${TARGETS_FILE}" >&2
  exit 1
fi

echo "sweep config:"
echo "  targets file  : ${TARGETS_FILE} (${TOTAL} targets)"
echo "  datasets dir  : ${DATASETS_DIR}"
echo "  output dir    : ${OUTPUT_DIR}"
echo "  timeout       : ${TIMEOUT}s"
echo "  binary        : ${BINARY}"
echo "  force re-run  : ${FORCE}"
echo

run_one() {
  local name="$1" dir="$2" pkg="$3" service="$4" codebase_pkg_prefix="$5" drop_patterns="$6"
  local target_dir="${OUTPUT_DIR}/${name}"
  local log="${target_dir}/driver_log.txt"

  if [[ "${FORCE}" -eq 0 && -s "${log}" ]] && grep -q 'Finished OSS pipeline evaluation' "${log}" 2>/dev/null; then
    echo "[skip] ${name} (already done: ${log})"
    return 200
  fi

  if [[ ! -d "${DATASETS_DIR}/${dir}" ]]; then
    echo "[skip-missing-dataset] ${name} (${DATASETS_DIR}/${dir} missing; run clone_datasets.sh)"
    return 201
  fi

  mkdir -p "${target_dir}"

  local cmd=(
    timeout "${TIMEOUT}"
    "${BINARY}"
    -dataset-root="${DATASETS_DIR}"
    -target-name="${name}"
    -target-dir="${dir}"
    -target-pkg="${pkg}"
    -service-name="${service}"
    -codebase-pkg-prefix="${codebase_pkg_prefix}"
    -output-dir="${target_dir}"
  )
  if [[ -n "${drop_patterns}" ]]; then
    cmd+=(-drop-exclude-package-patterns="${drop_patterns}")
  fi

  if [[ "${DRY_RUN}" -eq 1 ]]; then
    printf '[dry-run] '; printf '%q ' "${cmd[@]}"; printf '> %q 2>&1\n' "${log}"
    return 0
  fi

  echo "[run]  ${name} -> ${log}"
  local start=$SECONDS
  if "${cmd[@]}" >"${log}" 2>&1; then
    local elapsed=$((SECONDS - start))
    echo "[ok]   ${name} (${elapsed}s)"
    return 0
  else
    local rc=$?
    local elapsed=$((SECONDS - start))
    if [[ $rc -eq 124 ]]; then
      echo "[TIMEOUT] ${name} (${elapsed}s, exceeded ${TIMEOUT}s)"
    else
      echo "[FAIL] ${name} (${elapsed}s, exit=$rc)"
    fi
    return $rc
  fi
}

OK=0; FAIL=0; SKIP=0; SKIP_MISSING=0; TIMED_OUT=0
for i in "${!NAMES[@]}"; do
  idx=$((i + 1))
  name="${NAMES[$i]}"
  dir="${DIRS[$i]}"
  pkg="${PKGS[$i]}"
  service="${SERVICES[$i]}"
  codebase_pkg_prefix="${PREFIXES[$i]}"
  drop_patterns="${DROPS[$i]}"
  printf '\n=== [%d/%d] %s ===\n' "${idx}" "${TOTAL}" "${name}"
  set +e
  run_one "${name}" "${dir}" "${pkg}" "${service}" "${codebase_pkg_prefix}" "${drop_patterns}"
  rc=$?
  set -e
  case $rc in
    0)   OK=$((OK + 1));;
    124) TIMED_OUT=$((TIMED_OUT + 1));;
    200) SKIP=$((SKIP + 1));;
    201) SKIP_MISSING=$((SKIP_MISSING + 1));;
    *)   FAIL=$((FAIL + 1));;
  esac
done

echo
echo "=== sweep summary ==="
echo "  total              : ${TOTAL}"
echo "  ok                 : ${OK}"
echo "  skipped (done)     : ${SKIP}"
echo "  skipped (no data)  : ${SKIP_MISSING}"
echo "  failed             : ${FAIL}"
echo "  timed out          : ${TIMED_OUT}"
echo "  output             : ${OUTPUT_DIR}"
