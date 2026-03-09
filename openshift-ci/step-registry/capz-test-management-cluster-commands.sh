#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

source openshift-ci/capz-test-env.sh

# Phase 03: Management Cluster
# With USE_KUBECONFIG set, skips Kind creation and validates the external cluster.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
make _management_cluster RESULTS_DIR="${ARTIFACT_DIR}"
