#!/usr/bin/env bash
# Runs rpcshield RTA callgraph-construction benchmark across many services and
# many (flavor x worker-count) settings. Produces one <output_dir>/<service>.log
# per service, matching the layout in ~/prta_stats_review.
#
# Usage:
#   ./run_rta_sweep.sh -services FILE [options]
#
# Options:
#   -services FILE        Path to a file with one service name per line.
#                         Lines starting with '#' and blank lines are skipped.
#   -output DIR           Output directory for per-service logs.
#                         Default: /tmp/prta_stats_$(date +%Y%m%d_%H%M%S)
#   -flavors LIST         Comma-separated RTA flavors to benchmark.
#                         Default: srta,srta_kumo,srta_kumo_random,prta_kumo_nonblocking
#   -workers LIST         Comma-separated worker counts to benchmark.
#                         Default: 1,2,4,8,16,32,64
#   -timeout SEC          Per-service timeout (passed to `timeout` cmd).
#                         Default: 5400 (90m)
#   -workspace DIR        Go monorepo root. Default: $(git rev-parse --show-toplevel)
#   -force                Re-run services even if their .log already exists.
#   -dry-run              Print the commands that would be run, then exit.
#   -h | --help           Show this help.
#
# Env vars:
#   RPCSHIELD_REPO_REL_PATH  Path of the rpcshield repo root relative to -workspace, used to
#                            locate rpcshield.sh. Default: derived from this script's own
#                            location (this script lives at rpcshield/rta/evaluation_scripts/,
#                            two directories below the rpcshield repo root, wherever that repo is
#                            checked out inside -workspace). Override for unusual layouts (e.g.
#                            a symlinked checkout) where that derivation doesn't resolve.

set -euo pipefail

SERVICES_FILE=""
OUTPUT_DIR=""
FLAVORS="srta,srta_opt,srta_struct,srta_kumo,srta_kumo_random,prta_kumo_nonblocking"
WORKERS="1,2,4,8,16,32,64"
TIMEOUT=99999999
WORKSPACE=""
FORCE=0
DRY_RUN=0

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -services)  SERVICES_FILE="${2:?}"; shift 2;;
    -output)    OUTPUT_DIR="${2:?}";    shift 2;;
    -flavors)   FLAVORS="${2:?}";       shift 2;;
    -workers)   WORKERS="${2:?}";       shift 2;;
    -timeout)   TIMEOUT="${2:?}";       shift 2;;
    -workspace) WORKSPACE="${2:?}";     shift 2;;
    -force)     FORCE=1;                shift;;
    -dry-run)   DRY_RUN=1;              shift;;
    -h|--help)  usage 0;;
    *) echo "unknown arg: $1" >&2; usage 1;;
  esac
done

if [[ -z "${SERVICES_FILE}" ]]; then
  echo "error: -services FILE is required" >&2
  usage 1
fi
if [[ ! -f "${SERVICES_FILE}" ]]; then
  echo "error: services file not found: ${SERVICES_FILE}" >&2
  exit 1
fi

if [[ -z "${WORKSPACE}" ]]; then
  WORKSPACE="$(git rev-parse --show-toplevel)"
fi
# This script lives at rpcshield/rta/evaluation_scripts/, two directories below the rpcshield
# repo root -- derive that root's path relative to WORKSPACE instead of hardcoding it, so this
# works whether rpcshield is the workspace root itself (OSS standalone) or vendored arbitrarily
# deep inside a monorepo checkout.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RPCSHIELD_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
RPCSHIELD_REPO_REL_PATH="${RPCSHIELD_REPO_REL_PATH:-$(realpath --relative-to="${WORKSPACE}" "${RPCSHIELD_ROOT}")}"
RPCSHIELD_SH="${WORKSPACE}/${RPCSHIELD_REPO_REL_PATH}/rpcshield.sh"
if [[ ! -x "${RPCSHIELD_SH}" ]]; then
  echo "error: rpcshield.sh not found or not executable at ${RPCSHIELD_SH}" >&2
  exit 1
fi

if [[ -z "${OUTPUT_DIR}" ]]; then
  OUTPUT_DIR="/tmp/prta_stats_$(date +%Y%m%d_%H%M%S)"
fi
mkdir -p "${OUTPUT_DIR}"

# Read services into an array; strip comments and blanks.
mapfile -t SERVICES < <(grep -vE '^[[:space:]]*(#|$)' "${SERVICES_FILE}" | awk '{print $1}')
TOTAL=${#SERVICES[@]}
if [[ "${TOTAL}" -eq 0 ]]; then
  echo "error: no services found in ${SERVICES_FILE}" >&2
  exit 1
fi

echo "sweep config:"
echo "  services file : ${SERVICES_FILE} (${TOTAL} services)"
echo "  output dir    : ${OUTPUT_DIR}"
echo "  flavors       : ${FLAVORS}"
echo "  workers       : ${WORKERS}"
echo "  timeout       : ${TIMEOUT}s"
echo "  workspace     : ${WORKSPACE}"
echo "  force re-run  : ${FORCE}"
echo

run_one() {
  local svc="$1"
  local log="${OUTPUT_DIR}/${svc}.log"

  if [[ "${FORCE}" -eq 0 && -s "${log}" ]] && grep -q 'Finished CG construction-only evaluation' "${log}" 2>/dev/null; then
    echo "[skip] ${svc} (already done: ${log})"
    return 0
  fi

  local cmd=(
    timeout "${TIMEOUT}"
    "${RPCSHIELD_SH}" wpa
    -service="${svc}"
    -workspace="${WORKSPACE}"
    -loglevel=info
    -cg-construction-only
    -rta-bench-flavors="${FLAVORS}"
    -rta-bench-num-workers="${WORKERS}"
    -csv=false
    -html=false
  )

  if [[ "${DRY_RUN}" -eq 1 ]]; then
    printf '[dry-run] '; printf '%q ' "${cmd[@]}"; printf '> %q 2>&1\n' "${log}"
    return 0
  fi

  echo "[run]  ${svc} -> ${log}"
  local start=$SECONDS
  if (cd "${WORKSPACE}" && "${cmd[@]}") >"${log}" 2>&1; then
    local elapsed=$((SECONDS - start))
    echo "[ok]   ${svc} (${elapsed}s)"
  else
    local rc=$?
    local elapsed=$((SECONDS - start))
    if [[ $rc -eq 124 ]]; then
      echo "[TIMEOUT] ${svc} (${elapsed}s, exceeded ${TIMEOUT}s)"
    else
      echo "[FAIL] ${svc} (${elapsed}s, exit=$rc)"
    fi
    return $rc
  fi
}

OK=0; FAIL=0; SKIP=0; TIMED_OUT=0
i=0
for svc in "${SERVICES[@]}"; do
  i=$((i + 1))
  printf '\n=== [%d/%d] %s ===\n' "$i" "${TOTAL}" "${svc}"
  set +e
  run_one "${svc}"
  rc=$?
  set -e
  case $rc in
    0) if grep -q 'already done' <(echo "${svc}") 2>/dev/null; then SKIP=$((SKIP+1)); else OK=$((OK+1)); fi ;;
    124) TIMED_OUT=$((TIMED_OUT+1));;
    *)  FAIL=$((FAIL+1));;
  esac
done

echo
echo "=== sweep summary ==="
echo "  total    : ${TOTAL}"
echo "  ok       : ${OK}"
echo "  skipped  : ${SKIP}"
echo "  failed   : ${FAIL}"
echo "  timed out: ${TIMED_OUT}"
echo "  output   : ${OUTPUT_DIR}"
