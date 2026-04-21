#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

source openshift-ci/capz-test-env.sh

# Install gotestsum for JUnit XML output
GOFLAGS='' go install gotest.tools/gotestsum@v1.13.0
export PATH="${GOBIN:-$(go env GOPATH)/bin}:${PATH}"

# Run the full e2e test suite (Phases 01-07)
# Produces JUnit XML in ${ARTIFACT_DIR} for Prow to collect
gotestsum --junitfile="${ARTIFACT_DIR}/junit-e2e.xml" -- \
  -v ./test -count=1 -timeout 150m
