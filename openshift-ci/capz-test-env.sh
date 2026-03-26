# Shared environment variables for all CAPZ test steps in Prow.
# Sourced by each step command script to ensure consistent configuration.
# Edit this file to change test parameters for all phases at once.

# Source Azure credentials from the CI cluster profile (when available).
# CLUSTER_PROFILE_DIR is set by Prow when cluster_profile is configured.
if [[ -n "${CLUSTER_PROFILE_DIR:-}" && -f "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json" ]]; then
  # Suppress xtrace to prevent credential values from appearing in build logs
  { set +o xtrace; } 2>/dev/null
  AZURE_CLIENT_ID=$(jq -r .clientId "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json")
  AZURE_CLIENT_SECRET=$(jq -r .clientSecret "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json")
  AZURE_TENANT_ID=$(jq -r .tenantId "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json")
  AZURE_SUBSCRIPTION_ID=$(jq -r .subscriptionId "${CLUSTER_PROFILE_DIR}/osServicePrincipal.json")
  export AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID
  echo "[capz-test-env] Azure credentials loaded from cluster profile"
  set -o xtrace
fi

export INFRA_PROVIDER=aro
export CAPI_USER=prow
export DEPLOYMENT_ENV=ci
export REGION="${LEASED_RESOURCE:-uksouth}"
export OPERATORS_UAMIS_SUFFIX_FILE="/tmp/operators-uamis-suffix.txt"
export ARO_REPO_URL="https://github.com/marek-veber/cluster-api-installer.git"
export ARO_REPO_BRANCH="capi-tests"
export ARO_REPO_DIR="/tmp/cluster-api-installer-aro"

# Use the IPI-provisioned cluster kubeconfig (when available).
if [[ -n "${SHARED_DIR:-}" ]]; then
  export USE_KUBECONFIG="${SHARED_DIR}/kubeconfig"
fi

# Controllers are installed via deploy-charts.sh into standard namespaces
# (capi-system, capz-system), not MCE's multicluster-engine namespace.
export USE_K8S=false

# ARO HCP provisioning can take 60+ minutes in CI; increase from default 60m.
export DEPLOYMENT_TIMEOUT=90m

# Randomize NAME_PREFIX per run to avoid Azure Key Vault name collisions.
# KV names are globally unique with mandatory soft-delete — reusing a static
# name fails with VaultAlreadyExists if a previous run's vault wasn't purged.
# The first step to source this file generates the suffix; subsequent steps
# read the same value from SHARED_DIR.
NAME_PREFIX_FILE="${SHARED_DIR:-/tmp}/name-prefix"
if [[ -f "$NAME_PREFIX_FILE" ]]; then
  export NAME_PREFIX=$(cat "$NAME_PREFIX_FILE")
else
  export NAME_PREFIX="${WORKLOAD_CLUSTER_NAME:-capz-tests}-$(openssl rand -hex 2)"
  echo "$NAME_PREFIX" > "$NAME_PREFIX_FILE"
fi

# WORKLOAD_CLUSTER_NAMESPACE is set at the steps.env level in the ci-operator
# config, so all steps share the same fixed namespace without needing to pass
# it through SHARED_DIR files.
