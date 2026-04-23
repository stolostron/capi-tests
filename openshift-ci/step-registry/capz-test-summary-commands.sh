#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

source openshift-ci/capz-test-env.sh

# Collect all JUnit XMLs from SHARED_DIR into ARTIFACT_DIR
if compgen -G "${SHARED_DIR}/junit-*.xml" > /dev/null; then
  cp "${SHARED_DIR}"/junit-*.xml "${ARTIFACT_DIR}/"
fi

# make summary already calls enrich-junit-xml.sh + generate-summary.sh
make summary LATEST_RESULTS_DIR="${ARTIFACT_DIR}"
