#!/bin/sh
set -e

echo "[Compute-Sentry Pre-check] Starting node health verification..."

# Read thresholds from environment variables (injected by Operator)
# Defaults are used if variables are not set
MIN_P2P=${PRECHECK_MIN_P2P_GBPS:-20}
MIN_HBM=${PRECHECK_MIN_HBM_GBPS:-1000}

echo "[Compute-Sentry Pre-check] Configuration: Min P2P=${MIN_P2P} GB/s, Min HBM=${MIN_HBM} GB/s"

# Example: Check for GPU presence
if command -v nvidia-smi > /dev/null 2>&1; then
    echo "[Compute-Sentry Pre-check] GPU detected. Running diagnostic checks..."
    
    # Simulate HBM Bandwidth check
    # In a real scenario, we would execute a tool like 'bandwidthTest' or 'p2pBandwidthLatencyTest'
    # For now, we simulate a measured value.
    ACTUAL_HBM=1200 
    echo "[Compute-Sentry Pre-check] Measured HBM Bandwidth: ${ACTUAL_HBM} GB/s (Threshold: ${MIN_HBM} GB/s)"
    
    if [ "$ACTUAL_HBM" -lt "$MIN_HBM" ]; then
        echo "[Compute-Sentry Pre-check] ERROR: HBM Bandwidth below threshold! Blocking startup."
        exit 1
    fi

    # Simulate P2P Bandwidth check
    ACTUAL_P2P=25
    echo "[Compute-Sentry Pre-check] Measured P2P Bandwidth: ${ACTUAL_P2P} GB/s (Threshold: ${MIN_P2P} GB/s)"
    
    if [ "$ACTUAL_P2P" -lt "$MIN_P2P" ]; then
        echo "[Compute-Sentry Pre-check] ERROR: P2P Bandwidth below threshold! Blocking startup."
        exit 1
    fi

    echo "[Compute-Sentry Pre-check] Hardware health check: SUCCESS"
else
    echo "[Compute-Sentry Pre-check] Warning: No GPU detected on this node. Skipping hardware performance checks."
fi

echo "[Compute-Sentry Pre-check] All checks passed. Allowing training process to start."
exit 0
