#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Phase 03: Management Cluster
# Deploys Kind cluster with CAPI/CAPZ/ASO controllers or validates external cluster.
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
export ARO_REPO_DIR="${SHARED_DIR}/cluster-api-installer-aro"
make _management_cluster RESULTS_DIR="${ARTIFACT_DIR}"
