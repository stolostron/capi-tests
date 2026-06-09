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
  for var in AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID; do
    if [[ -z "${!var}" || "${!var}" == "null" ]]; then
      echo "[capz-test-env] ERROR: ${var} is missing or null in ${CLUSTER_PROFILE_DIR}/osServicePrincipal.json" >&2
      exit 1
    fi
  done
  export AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID
  echo "[capz-test-env] Azure credentials loaded from cluster profile"
  set -o xtrace
fi

# Override with CAPZ-specific Azure credentials from Vault (when available).
# These credentials target the subscription where workload clusters are deployed,
# which differs from the IPI cluster profile subscription used for the management cluster.
# The vault secret is mounted by Prow via the credentials block in each step ref YAML.
# Note: ipi-azure-post reads directly from CLUSTER_PROFILE_DIR/osServicePrincipal.json,
# so overriding env vars here does not affect management cluster deprovisioning.
CAPZ_CREDS_DIR="/var/run/capz-azure-credentials"
if [[ -d "${CAPZ_CREDS_DIR}" && -f "${CAPZ_CREDS_DIR}/AZURE_CLIENT_ID" ]]; then
  { set +o xtrace; } 2>/dev/null
  AZURE_CLIENT_ID=$(cat "${CAPZ_CREDS_DIR}/AZURE_CLIENT_ID")
  AZURE_CLIENT_SECRET=$(cat "${CAPZ_CREDS_DIR}/AZURE_CLIENT_SECRET")
  AZURE_TENANT_ID=$(cat "${CAPZ_CREDS_DIR}/AZURE_TENANT_ID")
  AZURE_SUBSCRIPTION_ID=$(cat "${CAPZ_CREDS_DIR}/AZURE_SUBSCRIPTION_ID")
  for var in AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID; do
    if [[ -z "${!var}" || "${!var}" == "null" ]]; then
      echo "[capz-test-env] ERROR: ${var} is missing or null in ${CAPZ_CREDS_DIR}" >&2
      exit 1
    fi
  done
  export AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID
  echo "[capz-test-env] Azure credentials overridden with CAPZ vault credentials"
  set -o xtrace
fi

: "${INFRA_PROVIDER:=aro}"
: "${CAPI_USER:=prow}"
: "${DEPLOYMENT_ENV:=ci}"
export INFRA_PROVIDER CAPI_USER DEPLOYMENT_ENV
# ARO HCP is only available in a limited set of regions; LEASED_RESOURCE is
# chosen by Prow from the azure4 pool and may not be one of them.  Hardcode
# to uksouth so the workload cluster always lands in a supported region.
# The IPI management cluster still uses LEASED_RESOURCE via ipi-azure-pre.
: "${REGION:=uksouth}"
export REGION
: "${OPERATORS_UAMIS_SUFFIX_FILE:=/tmp/operators-uamis-suffix.txt}"
export OPERATORS_UAMIS_SUFFIX_FILE

# Use pre-existing MSI identities from a Boskos-leased container resource group.
# LEASED_MSI_CONTAINERS is set by Prow from the aro-hcp-test-msi-containers-stg pool;
# each value is an Azure RG pre-provisioned with bare-named managed identities.
# CAPZ leases one container per job — take the first (and only) entry.
if [[ -n "${LEASED_MSI_CONTAINERS:-}" ]]; then
  read -r MSI_RESOURCEGROUPNAME _ <<< "${LEASED_MSI_CONTAINERS}"
  export MSI_RESOURCEGROUPNAME
  echo "[capz-test-env] Using pre-existing MSI from Boskos pool: ${MSI_RESOURCEGROUPNAME}"
fi
: "${ARO_REPO_URL:=https://github.com/stolostron/cluster-api-installer.git}"
: "${ARO_REPO_BRANCH:=main}"
: "${ARO_REPO_DIR:=/tmp/cluster-api-installer-aro}"
export ARO_REPO_URL ARO_REPO_BRANCH ARO_REPO_DIR

# Use the IPI-provisioned cluster kubeconfig (when available).
if [[ -n "${SHARED_DIR:-}" && -z "${USE_KUBECONFIG:-}" ]]; then
  export USE_KUBECONFIG="${SHARED_DIR}/kubeconfig"
fi

# Controllers are installed via deploy-charts.sh into standard namespaces
# (capi-system, capz-system), not MCE's multicluster-engine namespace.
: "${USE_K8S:=false}"
export USE_K8S

# ARO HCP provisioning can take 60+ minutes in CI; increase from default 60m.
: "${DEPLOYMENT_TIMEOUT:=90m}"
export DEPLOYMENT_TIMEOUT

# Randomize NAME_PREFIX per run to avoid Azure Key Vault name collisions.
# KV names are globally unique with mandatory soft-delete — reusing a static
# name fails with VaultAlreadyExists if a previous run's vault wasn't purged.
# The first step to source this file generates the suffix; subsequent steps
# read the same value from SHARED_DIR.
# NOTE: NAME_PREFIX must be ≤11 chars because ARO HCP node pool names
# (${NAME_PREFIX}-mp1) are limited to 15 characters.
NAME_PREFIX_FILE="${SHARED_DIR:-/tmp}/name-prefix"
if [[ -f "$NAME_PREFIX_FILE" ]]; then
  export NAME_PREFIX=$(cat "$NAME_PREFIX_FILE")
else
  export NAME_PREFIX="capz-$(openssl rand -hex 3)"
  echo "$NAME_PREFIX" > "$NAME_PREFIX_FILE"
fi

# Randomize RESOURCEGROUPNAME per run to prevent parallel jobs from
# interfering with each other's Azure resource groups.
# Same SHARED_DIR pattern as NAME_PREFIX above.
RESOURCEGROUPNAME_FILE="${SHARED_DIR:-/tmp}/resource-group-name"
if [[ -f "$RESOURCEGROUPNAME_FILE" ]]; then
  RESOURCEGROUPNAME="$(cat "$RESOURCEGROUPNAME_FILE")"
  export RESOURCEGROUPNAME
else
  RESOURCEGROUPNAME="capz-tests-$(openssl rand -hex 3)-resgroup"
  export RESOURCEGROUPNAME
  echo "$RESOURCEGROUPNAME" > "$RESOURCEGROUPNAME_FILE"
fi

# WORKLOAD_CLUSTER_NAMESPACE is set at the steps.env level in the ci-operator
# config, so all steps share the same fixed namespace without needing to pass
# it through SHARED_DIR files.
