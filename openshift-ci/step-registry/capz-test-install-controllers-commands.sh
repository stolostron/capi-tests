#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Source shared environment (Azure creds, kubeconfig, repo config)
source openshift-ci/capz-test-env.sh

# Clone the cluster-api-installer repository if not already present
if [[ -d "${ARO_REPO_DIR}" ]]; then
  echo "Repository already cloned at ${ARO_REPO_DIR}, skipping clone."
else
  echo "Cloning cluster-api-installer (branch: ${ARO_REPO_BRANCH})..."
  git clone --branch "${ARO_REPO_BRANCH}" --depth 1 "${ARO_REPO_URL}" "${ARO_REPO_DIR}"
fi

# Deploy CAPI/CAPZ/ASO controllers to the existing cluster
# DO_INIT_KIND=false - do NOT create a Kind cluster (we use CI-provisioned cluster)
# DO_DEPLOY=true     - deploy the Helm charts
# DO_CHECK=false     - skip built-in check (our e2e tests validate controllers)
export DO_INIT_KIND=false
export DO_DEPLOY=true
export DO_CHECK=false

cd "${ARO_REPO_DIR}"

# Set OCP_CONTEXT so deploy-charts.sh uses the current kubeconfig context
# instead of defaulting to "crc-admin" (which doesn't exist on IPI clusters).
export OCP_CONTEXT
OCP_CONTEXT=$(kubectl config current-context)
echo "Using kube context: ${OCP_CONTEXT}"

echo "Deploying CAPI/CAPZ controllers..."
bash scripts/deploy-charts.sh cluster-api cluster-api-provider-azure

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

# Create imagePullSecrets for quay.io/acm-d/ private registry
# The credentials are mounted from the CI vault by the step ref YAML
echo "Creating imagePullSecrets for quay.io/acm-d/..."
QUAY_AUTH=$(cat /etc/quay-pull-credentials/auth)

for NS in capi-system capz-system; do
  cat <<EOSECRET | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: acm-d-pull-secret
  namespace: ${NS}
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: '{"auths":{"quay.io":{"auth":"${QUAY_AUTH}"}}}'
EOSECRET
done

# Patch the controller-specific ServiceAccounts to use the pull secret
PATCH='{"imagePullSecrets": [{"name": "acm-d-pull-secret"}]}'
kubectl patch serviceaccount capi-manager -n capi-system -p "$PATCH"
kubectl patch serviceaccount mce-labeling-manager -n capi-system -p "$PATCH"
kubectl patch serviceaccount capz-manager -n capz-system -p "$PATCH"
kubectl patch serviceaccount azureserviceoperator-default -n capz-system -p "$PATCH"

# Restart deployments to pick up the new pull secrets
echo "Restarting controller deployments..."
kubectl rollout restart deployment -n capi-system
kubectl rollout restart deployment -n capz-system

echo "Controllers installed successfully."
