#!/usr/bin/env bash
# Picks the shard to run on this host from the milind-pmu cluster
# (4 hosts, 4 shards) and generates this host's shard on the fly from
# services.txt — round-robin (every Nth line where N is the cluster
# size). Each host runs its assigned shard sequentially.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-${HOME}/june_22_rta_mem_opt}"
SERVICES_FILE="${SCRIPT_DIR}/services.txt"
NUM_SHARDS=4

case "$(hostname)" in
  milind-pmu)   SHARD=0 ;;
  milind-pmu-2) SHARD=1 ;;
  milind-pmu-3) SHARD=2 ;;
  milind-pmu-4) SHARD=3 ;;
  *)
    echo "error: unknown hostname '$(hostname)'; expected milind-pmu[-2|-3|-4]" >&2
    exit 1
    ;;
esac

if [[ ! -r "${SERVICES_FILE}" ]]; then
  echo "error: services file not readable: ${SERVICES_FILE}" >&2
  exit 1
fi

# Generate this host's shard from services.txt — round-robin pick of
# every NUM_SHARDS-th line (offset SHARD), skipping comments and blanks.
SHARD_FILE="$(mktemp -t "rta_shard_${SHARD}.XXXXXX")"
trap 'rm -f "${SHARD_FILE}"' EXIT
awk -v shard="${SHARD}" -v n="${NUM_SHARDS}" '
  /^[[:space:]]*(#|$)/ {next}
  { if ((i++ % n) == shard) print }
' "${SERVICES_FILE}" > "${SHARD_FILE}"

count=$(wc -l < "${SHARD_FILE}")
echo "host=$(hostname) shard=${SHARD}/${NUM_SHARDS} services=${count} output=${OUTPUT_DIR}"

exec "${SCRIPT_DIR}/run_rta_sweep.sh" -services "${SHARD_FILE}" -output "${OUTPUT_DIR}"
