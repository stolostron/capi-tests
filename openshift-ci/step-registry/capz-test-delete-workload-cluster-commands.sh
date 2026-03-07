#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Phase 07: Delete Workload Cluster
# Deletes the workload cluster and verifies resource cleanup.
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
export ARO_REPO_DIR="${SHARED_DIR}/cluster-api-installer-aro"
make _delete-workload-cluster RESULTS_DIR="${ARTIFACT_DIR}"
