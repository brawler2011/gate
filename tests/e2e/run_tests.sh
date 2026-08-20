#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

TIER="${1:-all}"
echo "========================================================"
echo " Gate Observability E2E Test Runner"
echo " Target Tier: ${TIER}"
echo " Repo Root:   ${REPO_ROOT}"
echo "========================================================"

cd "${SCRIPT_DIR}"

case "${TIER}" in
  tier1|1)
    echo "Running Tier 1: Feature Coverage Tests..."
    go test -v -run "TestTier1" ./...
    ;;
  tier2|2)
    echo "Running Tier 2: Boundary & Corner Case Tests..."
    go test -v -run "TestTier2" ./...
    ;;
  tier3|3)
    echo "Running Tier 3: Cross-Feature Combination Tests..."
    go test -v -run "TestTier3" ./...
    ;;
  tier4|4)
    echo "Running Tier 4: Real-World Workload Scenarios..."
    go test -v -run "TestTier4" ./...
    ;;
  all|*)
    echo "Running All Tiers (1-4)..."
    go test -v -count=1 ./...
    ;;
esac

echo "========================================================"
echo " All E2E Observability Tests PASSED successfully!"
echo "========================================================"
