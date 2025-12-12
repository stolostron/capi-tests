# Useful Commands for Checking Resource Status During Test Runs

## ASO Logs

```bash
kubectl --context kind-capz-tests-stage logs -n capz-system deployment/azureserviceoperator-controller-manager --tail=100
```

To filter for specific content:

```bash
# Errors only
kubectl --context kind-capz-tests-stage logs -n capz-system deployment/azureserviceoperator-controller-manager --tail=100 | grep -i error

# Resource group related
kubectl --context kind-capz-tests-stage logs -n capz-system deployment/azureserviceoperator-controller-manager | grep -i resourcegroup

# Follow logs in real-time
kubectl --context kind-capz-tests-stage logs -n capz-system deployment/azureserviceoperator-controller-manager -f
```

## CAPZ Controller Logs

```bash
kubectl --context kind-capz-tests-stage logs -n capz-system deployment/capz-controller-manager --tail=100
```

To filter for specific content:

```bash
# Follow logs in real-time
kubectl --context kind-capz-tests-stage logs -n capz-system deployment/capz-controller-manager -f
```

## CAPI Controller Logs

```bash
kubectl --context kind-capz-tests-stage logs -n capi-system deployment/capi-controller-manager --tail=100
```

## Cluster Status

```bash
kubectl --context kind-capz-tests-stage get clusters -A
```

To get more details:

```bash
# Cluster YAML
kubectl --context kind-capz-tests-stage get cluster -n default -o yaml

# Cluster conditions
clusterctl describe cluster <cluster-name> --show-conditions=all
```

## ARO Resources

```bash
kubectl --context kind-capz-tests-stage get arocontrolplanes,aroclusters,aromachinepools -A
```

To get more details:

```bash
# ARO control plane YAML
kubectl --context kind-capz-tests-stage get arocontrolplane -n default -o yaml
```

## ASO Azure Resources

```bash
kubectl --context kind-capz-tests-stage get resourcegroups.resources.azure.com -A
kubectl --context kind-capz-tests-stage get virtualnetworks.network.azure.com -A
kubectl --context kind-capz-tests-stage get networksecuritygroups.network.azure.com -A
kubectl --context kind-capz-tests-stage get userassignedidentities.managedidentity.azure.com -A
kubectl --context kind-capz-tests-stage get vault.keyvault.azure.com -A
kubectl --context kind-capz-tests-stage get roleassignments.authorization.azure.com -A
```

## Azure CLI Verification

```bash
# Verify resource group exists in Azure
az group show --name <resource-group-name>

# List all resource groups
az group list --output table
```

## Events

```bash
kubectl --context kind-capz-tests-stage get events -n default --sort-by='.lastTimestamp' | tail -30
```

## Kind Cluster

```bash
# List kind clusters
kind get clusters

# List all pods
kubectl --context kind-capz-tests-stage get pods -A
```

## Secrets

```bash
kubectl --context kind-capz-tests-stage get secrets -n default
kubectl --context kind-capz-tests-stage get azureclusteridentity -n default
```
