#!/usr/bin/env bash
#
# Runs a DCGM diagnostic at the level given by DCGM_DIAG_LEVEL. Baked into the
# nvidia test image and invoked directly as the dcgm-diag-job Job body by the
# dcgm-diag feature (test/cases/nvidia/dcgm_test.go).
#
# Expects in the environment:
#   DCGM_DIAG_LEVEL   dcgmi diag run level (1-4)
#   EC2_INSTANCE_TYPE instance type, for logging only

set -o pipefail

echo "=== instance: ${EC2_INSTANCE_TYPE:-unknown} ==="
nvidia-smi --query-gpu=index,name,pci.bus_id --format=csv || true

# dcgmi diag drives every GPU on the node. Another process holding a
# GPU makes the run either fail or report misleading results, so
# assert exclusivity before spending an hour on it.
busy=$(nvidia-smi --query-compute-apps=pid,name,used_memory --format=csv,noheader)
if [ -n "$busy" ]; then
  echo "FAIL: other processes hold a GPU; dcgmi diag needs exclusive access:"
  echo "$busy"
  exit 1
fi

# Reuse the per-GPU-model overrides the level-2 run already relies on
# (e.g. L4 wired to PCIe x8 on single-GPU g6 sizes). Resolved relative to this
# script so it works wherever the image places gpu_unit_tests.
config="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/tests/dcgm-diag.yaml"
if [ ! -f "$config" ]; then
  echo "FAIL: expected DCGM config at $config"
  exit 1
fi

echo "=== dcgmi diag -r ${DCGM_DIAG_LEVEL} ==="
echo "measured level-4 durations: ~32 min on a 1-GPU instance,"
echo "~80 min on an 8-GPU instance. Scales with GPU count,"
echo "sub-linearly; total HBM has little effect."
echo
echo "note: a 'Skip' in the result table below is usually NVIDIA"
echo "policy rather than a packaging problem with this image. Some"
echo "plugins are enabled per GPU SKU in nvvs's built-in config and"
echo "skip silently where the SKU is not listed. Confirm the SKU is"
echo "actually permitted before suspecting missing plugins."
started=$(date +%s)
echo "start: $(date -Is)"

# dcgmi can go quiet for tens of minutes inside a single subtest, so
# emit a heartbeat carrying GPU telemetry. Utilisation and power are
# what distinguish "still working" from "hung": a stuck run shows 0%
# utilisation and idle power draw while the process stays alive.
(
  while :; do
    sleep 60
    echo "--- heartbeat +$(( $(date +%s) - started ))s ---"
    nvidia-smi --query-gpu=index,utilization.gpu,memory.used,power.draw,temperature.gpu,clocks_throttle_reasons.active \
      --format=csv,noheader 2>&1 || echo "  (nvidia-smi query failed)"
  done
) &
heartbeat=$!
# disown so bash does not print a job-termination notice (which dumps
# the whole subshell body) when the heartbeat is killed below.
disown "$heartbeat" 2>/dev/null || true
trap 'kill "$heartbeat" 2>/dev/null || true' EXIT

# stdbuf forces line buffering so dcgmi's per-test progress reaches
# the log as it happens instead of sitting in a pipe buffer.
stdbuf -oL -eL dcgmi diag -r "${DCGM_DIAG_LEVEL}" -c "$config"
rc=$?

kill "$heartbeat" 2>/dev/null || true
echo "end: $(date -Is) (elapsed $(( $(date +%s) - started ))s, exit $rc)"
exit $rc
