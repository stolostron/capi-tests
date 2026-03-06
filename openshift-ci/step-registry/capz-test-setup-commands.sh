#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Phase 02: Setup
# Clones cluster-api-installer repository and verifies scripts.
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
make _setup RESULTS_DIR="${ARTIFACT_DIR}"
