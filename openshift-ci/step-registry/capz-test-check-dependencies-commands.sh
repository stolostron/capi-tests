#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Install gotestsum for JUnit XML output
go install gotest.tools/gotestsum@v1.13.0
export PATH="${GOBIN:-$(go env GOPATH)/bin}:${PATH}"

# Run Phase 01: Check Dependencies
# TEST_RESULTS_DIR tells the Go test code to write artifacts (commands.log, etc.) to ARTIFACT_DIR
# JUnit XML is written directly via --junitfile flag
# Both are collected by Prow from ARTIFACT_DIR
TEST_RESULTS_DIR="${ARTIFACT_DIR}" gotestsum --junitfile="${ARTIFACT_DIR}/junit-check-dep.xml" -- \
  -v ./test -count=1 -run TestCheckDependencies
