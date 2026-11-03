#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

source openshift-ci/capz-test-env.sh

# Pre-test cleanup: safety net for aborted previous jobs.
# Cleans up any leftover Azure resources (resource groups, ARM resources,
# soft-deleted Key Vaults, AD apps, SPs) before deploying new workload cluster CRs.
#
# Two passes needed: gen.sh overrides CS_CLUSTER_NAME with WORKLOAD_CLUSTER_NAME,
# so resources end up under a different prefix than what CS_CLUSTER_NAME defaults to.
# See: https://github.com/stolostron/capi-tests/issues/627
# RESOURCEGROUPNAME is set by capz-test-env.sh (unique per job) and propagated
# to make clean-azure so cleanup targets the correct resource group.
# capz-tests-ci is the persistent Prow credential, not a per-run test identity.
# Preserve it in both cleanup passes until cleanup is fully ownership-aware.
export CLEANUP_EXCLUDED_AD_APP_IDS="6c35b111-a695-4e3d-9f16-229c34bbe53d"
FORCE=1 make clean-azure || true
FORCE=1 CS_CLUSTER_NAME="${WORKLOAD_CLUSTER_NAME:-capz-tests}" make clean-azure || true
