#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Phase 08: Validate Cleanup
# Validates cleanup operations for local and Azure resources.
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
export ARO_REPO_DIR="${SHARED_DIR}/cluster-api-installer-aro"
make _validate-cleanup RESULTS_DIR="${ARTIFACT_DIR}"
