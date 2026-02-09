# ARO-HCP Resource Creation Analysis

Analysis of resources created during ARO-HCP deployment via CAPZ/ASO, covering both the Kubernetes Custom Resource perspective and the Azure ARM resource perspective.

---

# Part A: Kubernetes Resources (CAPI/ASO Custom Resources)

Based on live cluster data from namespace `capz-test-20260208-184931`.

## A.1. Logical Dependency View (what waits for what)

Resources are organized by dependency level. Each level requires the resources above it to exist first. Arrows show `ownerReference` relationships.

```
LEVEL 0 - YAML Inputs (no dependencies, applied by kubectl)
├── Secret/aso-credential                    (credentials.yaml)
├── Secret/cluster-identity-secret           (credentials.yaml)
├── AzureClusterIdentity/cluster-identity    (credentials.yaml)
├── ResourceGroup/rcape-stage-resgroup       (aro.yaml - AROCluster.spec.resources[])
└── Cluster/rcape-stage                      (aro.yaml - top-level CAPI resource)

LEVEL 1 - Direct children of Cluster or ResourceGroup
│
├─── owned by Cluster/rcape-stage:
│    ├── AROCluster/rcape-stage                    (infrastructure ref)
│    ├── AROControlPlane/rcape-stage-control-plane  (control plane ref)
│    └── MachinePool/rcape-stage-mp-0              (worker pool)
│
└─── owned by ResourceGroup/rcape-stage-resgroup:
     ├── VirtualNetwork/rcape-stage-vnet
     ├── NetworkSecurityGroup/rcape-stage-nsg
     ├── Vault/rcape-stage-kv
     └── UserAssignedIdentity (x13):
          ├── cp-control-plane
          ├── cp-cluster-api-azure
          ├── cp-cloud-controller-manager
          ├── cp-cloud-network-config
          ├── cp-disk-csi-driver
          ├── cp-file-csi-driver
          ├── cp-image-registry
          ├── cp-ingress
          ├── cp-kms
          ├── dp-disk-csi-driver
          ├── dp-file-csi-driver
          ├── dp-image-registry
          └── service-managed-identity

LEVEL 2 - Children of Level 1 resources
│
├─── owned by VirtualNetwork:
│    └── VirtualNetworksSubnet/rcape-stage-vnet-rcape-stage-subnet
│
├─── owned by MachinePool:
│    └── AROMachinePool/rcape-stage-mp-0
│
├─── RoleAssignments on NSG (owned by NetworkSecurityGroup, x5):
│    ├── cloudcontrollermanagerroleid-nsg
│    ├── hcpcontrolplaneoperatorroleid-nsg
│    ├── filestorageoperatorroleid-nsg (cp)
│    ├── filestorageoperatorroleid-nsg (dp)
│    └── hcpservicemanagedidentityroleid-nsg
│
├─── RoleAssignments on VNet (owned by VirtualNetwork, x3):
│    ├── networkoperatorroleid-vnet
│    ├── hcpcontrolplaneoperatorroleid-vnet
│    └── hcpservicemanagedidentityroleid-vnet
│
├─── RoleAssignments on Vault (owned by Vault, x1):
│    └── keyvaultcryptouserroleid-keyvault
│
└─── RoleAssignments on UserAssignedIdentities ("reader" grants, x12):
     ├── readerroleid-controlplanemi
     ├── readerroleid-clusterapiazuremi
     ├── readerroleid-cloudcontrollermanagermi
     ├── readerroleid-cloudnetworkconfigmi
     ├── readerroleid-diskcsidrivermi
     ├── readerroleid-filecsidrivermi
     ├── readerroleid-imageregistrymi
     ├── readerroleid-ingressmi
     ├── readerroleid-kmsmi
     ├── federatedcredentialsroleid-dpdiskcsidrivermi
     ├── federatedcredentialsroleid-dpfilecsidrivermi
     └── federatedcredentialsroleid-dpimageregistrymi

LEVEL 3 - Children of Level 2 resources
│
├─── RoleAssignments on Subnet (owned by VirtualNetworksSubnet, x7):
│    ├── cloudcontrollermanagerroleid-subnet
│    ├── networkoperatorroleid-subnet
│    ├── hcpclusterapiproviderroleid-subnet
│    ├── filestorageoperatorroleid-subnet (cp)
│    ├── filestorageoperatorroleid-subnet (dp)
│    ├── ingressoperatorroleid-subnet
│    └── hcpservicemanagedidentityroleid-subnet
│
└─── owned by AROControlPlane + ResourceGroup:
     └── HcpOpenShiftCluster/rcape-stage    (the actual Azure HCP cluster)

LEVEL 4 - Final resources (depend on HCP cluster)
│
└─── owned by AROMachinePool + HcpOpenShiftCluster:
     └── HcpOpenShiftClustersNodePool/w-uksouth-mp-0

LEVEL 5 - Controller-generated (identity mappings, kubeconfig)
│
├── ConfigMap/identity-map-* (x13)   - one per managed identity
└── Secret/rcape-stage-kubeconfig    - workload cluster access
```

### Summary of dependency chain:

```
credentials.yaml → aro.yaml
                                │
                    Cluster ────┤
                                ├── AROCluster
                                ├── AROControlPlane ──→ HcpOpenShiftCluster
                                └── MachinePool ──→ AROMachinePool ──→ HcpOpenShiftClustersNodePool

ResourceGroup ──┬── VNet ──→ Subnet ──→ RoleAssignments (x7)
                ├── NSG ──→ RoleAssignments (x5)
                ├── Vault ──→ RoleAssignment (x1)
                └── UserAssignedIdentities (x13) ──→ RoleAssignments (x12)
```

---

## A.2. Chronological View (sorted by creation time)

### T+0s (17:52:45) - Initial resource creation from YAML apply

| Resource | Name |
|----------|------|
| Secret | aso-credential |
| Secret | cluster-identity-secret |
| AzureClusterIdentity | cluster-identity |
| ResourceGroup | rcape-stage-resgroup |
| NetworkSecurityGroup | rcape-stage-nsg |
| VirtualNetwork | rcape-stage-vnet |
| VirtualNetworksSubnet | rcape-stage-vnet-rcape-stage-subnet |
| Vault | rcape-stage-kv |
| UserAssignedIdentity | cp-cluster-api-azure |
| UserAssignedIdentity | cp-control-plane |

### T+1s (17:52:46) - CAPI reconciliation creates child resources

| Resource | Name |
|----------|------|
| Cluster | rcape-stage |
| AROCluster | rcape-stage |
| AROControlPlane | rcape-stage-control-plane |
| MachinePool | rcape-stage-mp-0 |
| AROMachinePool | rcape-stage-mp-0 |
| UserAssignedIdentity (x11) | remaining managed identities |
| RoleAssignment (x28) | all role assignments |

### T+2s to T+22s (17:52:47 - 17:53:07) - Azure resources provisioning

| Time | Event |
|------|-------|
| T+2s | ResourceGroup provisioning starts |
| T+6s | ResourceGroup ready |
| T+9s | NSG created in Azure |
| T+9s | VNet created in Azure |
| T+21s | ResourceGroupReady condition = True |
| T+21s | NetworkSecurityGroupsReady = True |
| T+22s | SubnetsReady = True |
| T+22s | VNetReady = True |
| T+22s | VaultReady = True |
| T+23s | UserIdentitiesReady = True |

### T+23s to T+68s (17:53:08 - 17:53:53) - Role assignments reconciled in Azure

| Time | Event |
|------|-------|
| T+23-68s | All 28 RoleAssignments reconciled against Azure |
| T+47s | Identity-map ConfigMaps start appearing |
| T+58s | All identity-map ConfigMaps created (x13) |

### T+112s (17:54:37) - RoleAssignmentReady, HCP cluster creation begins

| Time | Event |
|------|-------|
| T+112s | RoleAssignmentReady = True |
| T+112s | HcpOpenShiftCluster/rcape-stage created |
| T+113s | Secret/rcape-stage-kubeconfig generated |

### T+10m+ (18:03:28) - HCP cluster ready

| Time | Event |
|------|-------|
| ~T+10m43s | HcpClusterReady = True (Succeeded) |
| ~T+10m53s | HcpOpenShiftClustersNodePool/w-uksouth-mp-0 created |

---

## A.3. Kubernetes Resource Count Summary

| Category | Count |
|----------|-------|
| CAPI resources (Cluster, AROCluster, AROControlPlane, MachinePool, AROMachinePool) | 5 |
| Azure infra (ResourceGroup, VNet, Subnet, NSG, Vault) | 5 |
| Managed Identities (UserAssignedIdentity) | 13 |
| Role Assignments | 28 |
| Azure HCP resources (HcpOpenShiftCluster, NodePool) | 2 |
| Secrets | 3 |
| ConfigMaps (identity maps + kube-root-ca) | 14 |
| AzureClusterIdentity | 1 |
| **Total K8s Resources** | **71** |

---

## A.4. Kubernetes Timeline Summary

```
0s        Apply credentials.yaml + aro.yaml
          ├── Secrets, AzureClusterIdentity, ResourceGroup, VNet, NSG, Vault, Subnet created
1s        CAPI reconciliation: Cluster → AROCluster, AROControlPlane, MachinePool
          └── All 13 UserAssignedIdentities + 28 RoleAssignments created
~22s      Azure infra ready: ResourceGroup, VNet, Subnet, NSG, Vault, Identities
~68s      All RoleAssignments reconciled in Azure
~112s     All conditions met → HcpOpenShiftCluster created (Azure API call)
~10m43s   HcpClusterReady = True
~10m53s   NodePool created → cluster fully operational
```

---

# Part B: Azure ARM Resources

Azure resources created during an ARO-HCP deployment, from the Azure Resource Manager perspective.
Based on live deployment observation (region: `uksouth`).

## B.1. Azure Resources by Category

### Resource Group

| Azure Resource Type | Name | Purpose |
|---|---|---|
| `Microsoft.Resources/resourceGroups` | `{prefix}-resgroup` | Container for all deployment resources |

### Networking (3 resources)

| Azure Resource Type | Name | Purpose |
|---|---|---|
| `Microsoft.Network/virtualNetworks` | `{prefix}-vnet` | Virtual network for cluster nodes |
| `Microsoft.Network/virtualNetworks/subnets` | `{prefix}-subnet` | Subnet within the VNet for node placement |
| `Microsoft.Network/networkSecurityGroups` | `{prefix}-nsg` | Network security rules for cluster traffic |

### Security (1 resource)

| Azure Resource Type | Name | Purpose |
|---|---|---|
| `Microsoft.KeyVault/vaults` | `{prefix}-kv` | Key Vault for etcd encryption (KMS) |

### Managed Identities (13 resources)

#### Control Plane Identities (9)

| Name | Role |
|---|---|
| `cp-control-plane` | HCP control plane operator |
| `cp-cluster-api-azure` | CAPZ provider for cluster management |
| `cp-cloud-controller-manager` | Azure cloud controller manager |
| `cp-cloud-network-config` | Network operator configuration |
| `cp-disk-csi-driver` | Persistent disk CSI driver |
| `cp-file-csi-driver` | Azure File CSI driver |
| `cp-image-registry` | Image registry storage access |
| `cp-ingress` | Ingress operator |
| `cp-kms` | Key Management Service (etcd encryption) |

#### Data Plane Identities (3)

| Name | Role |
|---|---|
| `dp-disk-csi-driver` | Worker node disk CSI driver |
| `dp-file-csi-driver` | Worker node file CSI driver |
| `dp-image-registry` | Worker node image registry access |

#### Service Identity (1)

| Name | Role |
|---|---|
| `service-managed-identity` | Service-level managed identity (grants reader + federated credential roles to other identities) |

### Role Assignments (28 resources)

Role assignments connect managed identities to Azure resources with specific permissions.

#### On Network Security Group (5 assignments)

| Identity | Role |
|---|---|
| cp-cloud-controller-manager | Cloud Controller Manager role on NSG |
| cp-control-plane | HCP Control Plane Operator role on NSG |
| cp-file-csi-driver | File Storage Operator role on NSG |
| dp-file-csi-driver | File Storage Operator role on NSG |
| service-managed-identity | HCP Service Managed Identity role on NSG |

#### On Virtual Network (3 assignments)

| Identity | Role |
|---|---|
| cp-cloud-network-config | Network Operator role on VNet |
| cp-control-plane | HCP Control Plane Operator role on VNet |
| service-managed-identity | HCP Service Managed Identity role on VNet |

#### On Subnet (7 assignments)

| Identity | Role |
|---|---|
| cp-cloud-controller-manager | Cloud Controller Manager role |
| cp-cloud-network-config | Network Operator role |
| cp-cluster-api-azure | HCP Cluster API Provider role |
| cp-file-csi-driver | File Storage Operator role |
| cp-ingress | Ingress Operator role |
| dp-file-csi-driver | File Storage Operator role |
| service-managed-identity | HCP Service Managed Identity role |

#### On Key Vault (1 assignment)

| Identity | Role |
|---|---|
| cp-kms | Key Vault Crypto User role |

#### Reader roles on Managed Identities (9 assignments)

Service-managed-identity grants Reader role to each control plane identity:
- cp-control-plane, cp-cluster-api-azure, cp-cloud-controller-manager
- cp-cloud-network-config, cp-disk-csi-driver, cp-file-csi-driver
- cp-image-registry, cp-ingress, cp-kms

#### Federated Credential roles on Data Plane Identities (3 assignments)

Service-managed-identity grants Federated Credentials role to:
- dp-disk-csi-driver, dp-file-csi-driver, dp-image-registry

### HCP Cluster (2 resources)

| Azure Resource Type | Name | Purpose |
|---|---|---|
| `Microsoft.RedHatOpenShift/HCPOpenShiftClusters` | `{prefix}` | The ARO-HCP hosted control plane cluster |
| `Microsoft.RedHatOpenShift/HCPOpenShiftClusters/NodePools` | `w-{region}-mp-0` | Worker node pool |

---

## B.2. Azure Resource Provisioning Timeline

All times relative to first `kubectl apply`. Based on ASO Ready condition timestamps.

```
T+0s     ─── kubectl apply (credentials.yaml, aro.yaml) ───

T+6s     ResourceGroup ................................. Succeeded    (6s to provision)
T+12s    NetworkSecurityGroup .......................... Succeeded    (12s)
T+16s    KeyVault ...................................... Succeeded    (16s)
T+22s    VirtualNetwork ................................ Succeeded    (22s)
T+35s    Subnet ........................................ Succeeded    (35s, waits for VNet)
T+47s    UserAssignedIdentity (first) .................. Succeeded    (47s)
T+58s    UserAssignedIdentity (last of 13) ............. Succeeded    (58s)

T+59s    ─── Role Assignments start resolving ───

T+59s    RoleAssignment (first - CP on NSG) ............ Succeeded
T+62s    RoleAssignment (CP on VNet) ................... Succeeded
T+66s    RoleAssignment (CP on Subnet) ................. Succeeded
T+68s    RoleAssignment (CCM on NSG + Subnet) .......... Succeeded
  ...    (parallel batch processing)
T+100s   RoleAssignment (file-csi, ingress, kms) ....... Succeeded
T+103s   RoleAssignment (service-mi reader grants) ..... Succeeded
T+105s   RoleAssignment (last of 28) ................... Succeeded

T+112s   ─── All prerequisites met, HCP cluster creation starts ───

T+112s   HcpOpenShiftCluster PUT request sent to Azure API

  ...    (Azure provisions hosted control plane internally)

T+18m53s HcpOpenShiftClustersNodePool .................. Succeeded
T+24m22s HcpOpenShiftCluster ........................... Reconciling (still finalizing)
```

### Provisioning Duration by Resource Type

| Resource Type | Count | First Ready | Last Ready | Avg Duration |
|---|---|---|---|---|
| ResourceGroup | 1 | T+6s | T+6s | ~6s |
| NetworkSecurityGroup | 1 | T+12s | T+12s | ~12s |
| KeyVault | 1 | T+16s | T+16s | ~16s |
| VirtualNetwork | 1 | T+22s | T+22s | ~22s |
| Subnet | 1 | T+35s | T+35s | ~13s (after VNet) |
| UserAssignedIdentity | 13 | T+47s | T+58s | ~3-4s each |
| RoleAssignment | 28 | T+59s | T+105s | ~2-3s each |
| HcpOpenShiftCluster | 1 | T+112s | ongoing | ~10-25min |
| NodePool | 1 | T+18m53s | T+18m53s | ~8min (after HCP ready enough) |

### Critical Path

```
ResourceGroup (6s)
  └─→ VirtualNetwork (22s)
       └─→ Subnet (35s)
            └─→ RoleAssignments on Subnet (68-105s)
                 └─→ HcpOpenShiftCluster (112s to start, ~25min total)
                      └─→ NodePool (~19min)
```

The bottleneck is the **HcpOpenShiftCluster** Azure API call, which takes the majority of the total deployment time (~10-25 minutes). All infrastructure resources (RG, VNet, NSG, Vault, Identities, Role Assignments) complete within ~105 seconds.

---

## B.3. Azure Resource Count Summary

| Category | Count | Azure Provider |
|---|---|---|
| Resource Group | 1 | `Microsoft.Resources` |
| Virtual Network | 1 | `Microsoft.Network` |
| Subnet | 1 | `Microsoft.Network` |
| Network Security Group | 1 | `Microsoft.Network` |
| Key Vault | 1 | `Microsoft.KeyVault` |
| User Assigned Identities | 13 | `Microsoft.ManagedIdentity` |
| Role Assignments | 28 | `Microsoft.Authorization` |
| HCP OpenShift Cluster | 1 | `Microsoft.RedHatOpenShift` |
| HCP Node Pool | 1 | `Microsoft.RedHatOpenShift` |
| **Total Azure Resources** | **48** | |

---

## B.4. Azure Dependency Graph

```
ResourceGroup
├── NetworkSecurityGroup
│   └── RoleAssignments (x5): CCM, ControlPlane, FileCSI-cp, FileCSI-dp, ServiceMI
├── VirtualNetwork
│   ├── RoleAssignments (x3): NetworkConfig, ControlPlane, ServiceMI
│   └── Subnet
│       └── RoleAssignments (x7): CCM, NetworkConfig, CAPZ, FileCSI-cp, FileCSI-dp, Ingress, ServiceMI
├── KeyVault
│   └── RoleAssignment (x1): KMS CryptoUser
├── UserAssignedIdentities (x13)
│   └── RoleAssignments (x12): 9x Reader + 3x FederatedCredentials
└── HcpOpenShiftCluster  (waits for ALL above)
    └── NodePool
```
