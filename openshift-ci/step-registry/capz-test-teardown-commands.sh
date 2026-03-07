#!/bin/bash
set -o nounset
set -o pipefail
set -o xtrace

# Teardown: Safety net cleanup (post step - always runs)
# Deletes Kind cluster, cloned repository, kubeconfig files, and Azure resources.
# Uses best_effort so cleanup failures do not mask test failures.
# Does NOT delete ${ARTIFACT_DIR} - only the local results/ directory.
export ARO_REPO_DIR="${SHARED_DIR}/cluster-api-installer-aro"
FORCE=1 make clean-all || true
