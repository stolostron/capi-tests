#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Phase 04: Generate YAMLs
# Generates credential and cluster YAML manifests for deployment.
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
make _generate-yamls RESULTS_DIR="${ARTIFACT_DIR}"
