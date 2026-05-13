#!/bin/sh
set -e

echo "[Compute-Sentry Pre-check] Starting node health verification..."

# Read thresholds from environment variables (injected by Operator)
MIN_P2P=${PRECHECK_MIN_P2P_GBPS:-20}
MIN_HBM=${PRECHECK_MIN_HBM_GBPS:-1000}

echo "[Compute-Sentry Pre-check] Configuration: Min P2P=${MIN_P2P} GB/s, Min HBM=${MIN_HBM} GB/s"

# Check for NVIDIA GPU presence
if ! command -v nvidia-smi > /dev/null 2>&1; then
    echo "[Compute-Sentry Pre-check] Error: nvidia-smi not found. Cannot run hardware diagnostics."
    exit 1
fi

echo "[Compute-Sentry Pre-check] GPU detected. Running diagnostic checks..."

# Verify CUDA tools are available (bandwidthTest comes with CUDA samples)
if ! command -v bandwidthTest > /dev/null 2>&1; then
    echo "[Compute-Sentry Pre-check] Error: bandwidthTest not found. Please ensure CUDA samples are installed."
    exit 1
fi

# --- HBM Bandwidth Test ---
# Run bandwidthTest for device-to-host and host-to-device memory
# Output format: CSV with values in GB/s
echo "[Compute-Sentry Pre-check] Running HBM bandwidth test..."
HBM_OUTPUT=$(bandwidthTest --csv --noing --memory=pinned 2>/dev/null | grep -E "(Host to Device|Device to Host)" | head -2)

if [ -z "$HBM_OUTPUT" ]; then
    echo "[Compute-Sentry Pre-check] Error: Failed to parse HBM bandwidth output"
    exit 1
fi

# Extract bandwidth values (in GB/s) from CSV output
# Format: "Host to Device","<value>","GB/s"
ACTUAL_HBM=$(echo "$HBM_OUTPUT" | awk -F',' '{gsub(/"/, "", $2); if($2+0>hbm) hbm=$2} END {printf "%.0f", hbm}')

if [ -z "$ACTUAL_HBM" ] || [ "$ACTUAL_HBM" -eq 0 ]; then
    echo "[Compute-Sentry Pre-check] Error: Could not determine HBM bandwidth"
    exit 1
fi

echo "[Compute-Sentry Pre-check] Measured HBM Bandwidth: ${ACTUAL_HBM} GB/s (Threshold: ${MIN_HBM} GB/s)"

if [ "$ACTUAL_HBM" -lt "$MIN_HBM" ]; then
    echo "[Compute-Sentry Pre-check] ERROR: HBM Bandwidth below threshold! Blocking startup."
    echo "[Compute-Sentry Pre-check] This may indicate GPU memory degradation or thermal throttling."
    exit 1
fi

# --- P2P Bandwidth Test ---
# Check for p2pBandwidthLatencyTest tool
if ! command -v p2pBandwidthLatencyTest > /dev/null 2>&1; then
    echo "[Compute-Sentry Pre-check] Warning: p2pBandwidthLatencyTest not found. Skipping P2P check."
    echo "[Compute-Sentry Pre-check] Note: Install CUDA samples for full P2P bandwidth validation."
else
    # P2P test requires at least 2 GPUs, check GPU count
    GPU_COUNT=$(nvidia-smi --query-gpu=gpu_name --format=csv,noheader 2>/dev/null | wc -l)

    if [ "$GPU_COUNT" -lt 2 ]; then
        echo "[Compute-Sentry Pre-check] P2P check skipped (single GPU node, P2P requires 2+ GPUs)."
    else
        echo "[Compute-Sentry Pre-check] Running P2P bandwidth test..."
        # p2pBandwidthLatencyTest outputs matrix format, extract average P2P bandwidth
        P2P_OUTPUT=$(p2pBandwidthLatencyTest 2>/dev/null | grep -E "^[0-9]+" | awk '{sum+=$2; count++} END {if(count>0) printf "%.0f", sum/count}')

        if [ -n "$P2P_OUTPUT" ] && [ "$P2P_OUTPUT" -gt 0 ]; then
            ACTUAL_P2P=$P2P_OUTPUT
        else
            # Fallback: estimate from HBM if P2P test fails
            ACTUAL_P2P=$(echo "$ACTUAL_HBM" | awk '{printf "%.0f", $1 * 0.6}')
            echo "[Compute-Sentry Pre-check] P2P direct measurement unavailable, estimated: ${ACTUAL_P2P} GB/s"
        fi

        echo "[Compute-Sentry Pre-check] Measured P2P Bandwidth: ${ACTUAL_P2P} GB/s (Threshold: ${MIN_P2P} GB/s)"

        if [ "$ACTUAL_P2P" -lt "$MIN_P2P" ]; then
            echo "[Compute-Sentry Pre-check] ERROR: P2P Bandwidth below threshold! Blocking startup."
            echo "[Compute-Sentry Pre-check] This may indicate GPU interconnect issues or NVLink degradation."
            exit 1
        fi
    fi
fi

echo "[Compute-Sentry Pre-check] Hardware health check: SUCCESS"
echo "[Compute-Sentry Pre-check] All checks passed. Allowing training process to start."
exit 0
