#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Source Azure credentials from the CI cluster profile
AZURE_CLIENT_ID=$(jq -r .clientId "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json")
AZURE_CLIENT_SECRET=$(jq -r .clientSecret "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json")
AZURE_TENANT_ID=$(jq -r .tenantId "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json")
AZURE_SUBSCRIPTION_ID=$(jq -r .subscriptionId "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json")
export AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID

# Use the CI-provisioned cluster via USE_KUBECONFIG
# This skips Kind cluster creation (Phase 03) and uses the external cluster
export USE_KUBECONFIG="${SHARED_DIR}/kubeconfig"

# Override USE_K8S=false to prevent auto-switch to MCE namespaces.
# Controllers installed via deploy-charts.sh use standard namespaces
# (capi-system, capz-system), not multicluster-engine.
export USE_K8S=false

# CI-specific naming to avoid collisions
export CAPI_USER=prow
export DEPLOYMENT_ENV=ci

# Point to the cloned cluster-api-installer repo from the install step
export ARO_REPO_DIR="/tmp/cluster-api-installer-aro"

# Install gotestsum for JUnit XML output
GOFLAGS='' go install gotest.tools/gotestsum@v1.13.0
export PATH="${GOBIN:-$(go env GOPATH)/bin}:${PATH}"

# Run the full e2e test suite (Phases 01-07)
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect
gotestsum --junitfile="${ARTIFACT_DIR}/junit-e2e.xml" -- \
  -v ./test -count=1 -timeout 150m
