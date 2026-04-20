#!/bin/bash
set -o nounset
set -o pipefail
set -o xtrace

# Teardown: Safety net cleanup (post step - always runs)
# 1. Revert MCE components to original state (skips if not an MCE cluster)
# 2. Delete Kind cluster, cloned repository, kubeconfig files, and Azure resources
# Uses best_effort so cleanup failures do not mask test failures.
# Does NOT delete ${ARTIFACT_DIR} - only the local results/ directory.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
make _mce-teardown RESULTS_DIR="${ARTIFACT_DIR}" || true
FORCE=1 make clean-all || true
