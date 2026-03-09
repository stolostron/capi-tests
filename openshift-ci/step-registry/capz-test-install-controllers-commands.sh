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

# Use the CI-provisioned cluster kubeconfig
export KUBECONFIG="${SHARED_DIR}/kubeconfig"

# Clone the cluster-api-installer repository
ARO_REPO_URL="${ARO_REPO_URL:-https://github.com/RadekCap/cluster-api-installer.git}"
ARO_REPO_BRANCH="${ARO_REPO_BRANCH:-ARO-ASO}"
ARO_REPO_DIR="/tmp/cluster-api-installer-aro"

echo "Cloning cluster-api-installer (branch: ${ARO_REPO_BRANCH})..."
git clone --branch "${ARO_REPO_BRANCH}" --depth 1 "${ARO_REPO_URL}" "${ARO_REPO_DIR}"

# Deploy CAPI/CAPZ/ASO controllers to the existing cluster
# DO_INIT_KIND=false - do NOT create a Kind cluster (we use CI-provisioned cluster)
# DO_DEPLOY=true     - deploy the Helm charts
# DO_CHECK=false     - skip built-in check (our e2e tests validate controllers)
export DO_INIT_KIND=false
export DO_DEPLOY=true
export DO_CHECK=false

cd "${ARO_REPO_DIR}"
echo "Deploying CAPI/CAPZ/ASO controllers..."
bash scripts/deploy-charts.sh cluster-api cluster-api-provider-azure azure-service-operator

# Patch ASO credentials secret with the Azure SP credentials
echo "Patching ASO credentials secret..."
kubectl create namespace azureserviceoperator-system --dry-run=client -o yaml | kubectl apply -f -

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: aso-controller-settings
  namespace: azureserviceoperator-system
stringData:
  AZURE_SUBSCRIPTION_ID: "${AZURE_SUBSCRIPTION_ID}"
  AZURE_TENANT_ID: "${AZURE_TENANT_ID}"
  AZURE_CLIENT_ID: "${AZURE_CLIENT_ID}"
  AZURE_CLIENT_SECRET: "${AZURE_CLIENT_SECRET}"
EOF

echo "Controllers installed successfully."
