#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Phase 05: Deploy CRs
# Applies cluster resources and waits for control plane deployment.
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
make _deploy-crs RESULTS_DIR="${ARTIFACT_DIR}"
