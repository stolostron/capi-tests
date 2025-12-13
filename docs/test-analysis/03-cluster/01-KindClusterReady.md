# Test 1: TestKindCluster_KindClusterReady

**Location:** `test/03_cluster_test.go:13-101`

**Purpose:** Deploy Kind cluster with CAPI/CAPZ/ASO controllers (or skip if already exists), then verify cluster is accessible via kubectl.

---

## Commands Executed

| Step | Command | Purpose |
|------|---------|---------|
| 1 | `kind get clusters` | Check if the management cluster already exists |
| 2 | `bash <repo>/scripts/deploy-charts-kind-capz.sh` | Deploy Kind cluster (only if cluster doesn't exist) |
| 3 | `kubectl --context kind-<cluster-name> get nodes` | Verify cluster is accessible |

---

## Detailed Flow

```
1. Check prerequisite: Does ARO_REPO_DIR exist?
   └─ No  → SKIP test
   └─ Yes → Continue

2. Run: kind get clusters
   └─ Output contains cluster name?
      └─ Yes → Skip deployment, go to step 4
      └─ No  → Continue to step 3

3. Run deployment script:
   - Set env: KIND_CLUSTER_NAME=<management-cluster-name>
   - cd to ARO_REPO_DIR
   - Run: bash scripts/deploy-charts-kind-capz.sh

4. Verify cluster:
   - Set env: KUBECONFIG=$HOME/.kube/config
   - Run: kubectl --context kind-<name> get nodes
```

---

## Key Variables

| Variable | Default Value |
|----------|---------------|
| `config.RepoDir` | `/tmp/cluster-api-installer-aro` |
| `config.ManagementClusterName` | `capz-tests-stage` |

---

## Deployment Script Breakdown

The `deploy-charts-kind-capz.sh` script calls 3 sub-scripts:

```
deploy-charts-kind-capz.sh
├── 1. setup-kind-cluster.sh
├── 2. deploy-charts.sh (cluster-api, cluster-api-provider-azure)
└── 3. wait-for-controllers.sh (capi, capz)
```

### Step 1: setup-kind-cluster.sh

| Command | Purpose |
|---------|---------|
| `kind get clusters` | Check if cluster exists |
| `kind create cluster --name $KIND_CLUSTER_NAME` | Create Kind cluster (if not exists) |
| `helm repo add jetstack https://charts.jetstack.io --force-update` | Add cert-manager Helm repo |
| `helm repo update` | Update Helm repos |
| `helm upgrade --install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set crds.enabled=true --wait --timeout 5m` | Install cert-manager |

### Step 2: deploy-charts.sh

Loops over each chart (`cluster-api`, `cluster-api-provider-azure`):

| Command | Purpose |
|---------|---------|
| `helm template charts/cluster-api --include-crds \| kubectl apply -f - --server-side --force-conflicts` | Deploy CAPI CRDs + controllers |
| `helm template charts/cluster-api-provider-azure --include-crds \| kubectl apply -f - --server-side --force-conflicts` | Deploy CAPZ CRDs + controllers |

### Step 3: wait-for-controllers.sh

For each controller (`capi`, `capz`):

| Command | Purpose |
|---------|---------|
| `kubectl events -n capi-system --watch &` | Stream events (background) |
| `kubectl -n capi-system wait deployment/capi-controller-manager --for condition=Available=True --timeout=10m` | Wait for CAPI ready |
| `kubectl events -n capz-system --watch &` | Stream events (background) |
| `kubectl -n capz-system wait deployment/capz-controller-manager --for condition=Available=True --timeout=10m` | Wait for CAPZ ready |

---

## Summary of All Commands

```
1.  kind get clusters
2.  kind create cluster --name <name>
3.  helm repo add jetstack https://charts.jetstack.io --force-update
4.  helm repo update
5.  helm upgrade --install cert-manager ...
6.  helm template charts/cluster-api | kubectl apply ...
7.  helm template charts/cluster-api-provider-azure | kubectl apply ...
8.  kubectl wait deployment/capi-controller-manager ...
9.  kubectl wait deployment/capz-controller-manager ...
10. kubectl --context kind-<name> get nodes
```
