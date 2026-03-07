#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

source openshift-ci/capz-test-env.sh

# Start Docker daemon (required for Kind cluster creation)
dockerd &>/tmp/dockerd.log &
echo "Waiting for Docker daemon to start..."
for i in $(seq 1 30); do
  if docker info &>/dev/null; then
    echo "Docker daemon is ready (took ${i}s)"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: Docker daemon failed to start. Log:"
    cat /tmp/dockerd.log
    exit 1
  fi
  sleep 1
done

# Phase 03: Management Cluster
# Deploys Kind cluster with CAPI/CAPZ/ASO controllers or validates external cluster.
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect.
export TEST_RESULTS_DIR="${ARTIFACT_DIR}"
make _management_cluster RESULTS_DIR="${ARTIFACT_DIR}"
