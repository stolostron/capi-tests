#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Phase 06: Verify Workload Cluster
# Validates the deployed workload cluster is accessible and healthy.
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
make _verify-workload-cluster RESULTS_DIR="${ARTIFACT_DIR}"
