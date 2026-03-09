#!/bin/bash
set -o nounset
set -o pipefail
set -o xtrace

source openshift-ci/capz-test-env.sh

# Teardown: Safety net cleanup (post step - always runs)
# Cleans up Azure resources created by the test suite (workload cluster, resource groups).
# The management cluster itself is deprovisioned by the ipi-azure-post chain.
# Uses best_effort so cleanup failures do not mask test failures.
FORCE=1 make clean-azure || true
