# ARO identity and role-assignment phases

`ARO_SPLIT_DEPLOYMENT=true` enables an opt-in deployment path for the current installer `aro.yaml` format. The default ARO and ROSA paths are unchanged.

The installer embeds resource groups, networking, Key Vault, user-assigned identities and role assignments in `AROCluster.spec.resources`. Applying that whole document immediately lets controllers create everything using the same credential. Splitting the YAML documents alone does not separate Azure privileges: the embedded resources must also be separated and explicitly bound to credentials.

## Credential boundary

| Work | Credential | Expected operator-provided permissions |
|------|------------|----------------------------------------|
| Resource group, networking and vault prerequisites | Deployment principal | Required infrastructure operations at the intended scopes |
| Create user-assigned identities | Identity principal | Identity creation at the intended identity scope |
| Create role assignments | Role-assignment principal | Role-assignment writes at the scopes already specified by the generated manifests |
| CAPI cluster deployment and subsequent reconciliation | Deployment principal | Cluster/infrastructure operations and read access to pre-created identities and assignments |

The suite does not create principals, define Azure roles, or grant permissions to these principals. An operator must provision their permissions. In particular, the deployment principal should lack identity creation and role-assignment write permissions at the tested scopes. Do not assume that a generic Contributor grant meets the identity-creation restriction.

The identity and role credentials are existing Secrets in the workload namespace. Every emitted ASO resource has an explicit `serviceoperator.azure.com/credential-from` annotation. The final cluster bundle uses the deployment credential for every embedded ASO resource; identities and role assignments additionally use `serviceoperator.azure.com/reconcile-policy: skip`. ASO still issues reads for `skip` resources, but does not issue Azure PUT/DELETE operations. The original generated YAML is preserved.

Preflight rejects missing credentials, duplicate secret names/client IDs, and a CAPZ `AzureClusterIdentity` that differs from the deployment principal. The control plane must reference that validated identity. These checks establish credential routing, **not proof of least privilege**. They do not inspect effective Azure RBAC grants, inherited grants or Kubernetes authorization. ASO can use privileged Secrets for resources in their namespace, so this is a trusted-runner test workflow, not a multitenant security boundary. Restrict access to that namespace and its Secrets.

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `ARO_SPLIT_DEPLOYMENT` | `false` | Enable the staged path in `_deploy-crs` / `test-all` |
| `ARO_IDENTITY_CREDENTIAL_SECRET` | Empty; required for creation | Secret for the distinct identity-creation principal |
| `ARO_ROLE_CREDENTIAL_SECRET` | Empty; required | Secret for the distinct role-assignment principal |
| `ARO_DEPLOYMENT_CREDENTIAL_SECRET` | `aso-credential` | Deployment credential, matching generated `AzureClusterIdentity.spec.clientID` |
| `ARO_PREEXISTING_IDENTITIES` | `false` | Read existing identities using the deployment credential instead of creating them |

`CLUSTER_DEPLOYMENT_TIMEOUT` bounds each entire phase, including credential reads, applies and readiness polling. Kubectl requests also have a 30-second request timeout. Both `Ready=True` and a condition observed generation matching the current object generation are required; stale conditions are polled until current or the phase deadline expires. Bootstrap bundles are not reapplied after a Cluster already exists, avoiding a switch back to elevated credentials during resumed runs.

## Execution

1. Set a stable `WORKLOAD_CLUSTER_NAMESPACE` before initializing the run. Generate manifests using the deployment principal, following the normal prerequisites, setup and management-cluster flow.
2. Create that namespace and provision the identity/role credential Secrets there through your normal credential-management process. ASO supports service-principal or workload-identity credential formats; this workflow checks their `AZURE_CLIENT_ID` values without reading client secrets. Generate `credentials.yaml` for the same deployment principal as `ARO_DEPLOYMENT_CREDENTIAL_SECRET`.
3. Enable the split mode and run the phases:

   ```bash
   export ARO_SPLIT_DEPLOYMENT=true
   export ARO_IDENTITY_CREDENTIAL_SECRET=identity-bootstrap
   export ARO_ROLE_CREDENTIAL_SECRET=role-bootstrap
   make _create-identities
   make _assign-roles
   make _deploy-cluster
   ```

   `_create-identities` applies the generated credentials, creates prerequisite infrastructure and waits for it, then creates/waits for identities. `_assign-roles` verifies prerequisite readiness before applying and waiting for roles. `_deploy-cluster` verifies all setup resources before applying the transformed CAPI manifests. Each phase can run in a separate process. The normal `_deploy-crs` target runs these phases in order when enabled, then performs its existing deployment monitoring.

Generated phase bundles are stored in `<installer-output>/split-deployment/` with private file permissions. Unsupported top-level resources or new prerequisite kinds fail before resource application; extend the parser and fixtures when the installer changes shape.

## Externally provisioned identities and cleanup

For externally provisioned identities, configure the installer `MSI_RESOURCEGROUPNAME` to the separate existing identity resource group, then set `ARO_PREEXISTING_IDENTITIES=true`. The identity phase applies read-only ASO references and waits for exported identity ConfigMaps. `ARO_IDENTITY_CREDENTIAL_SECRET` is not required in this mode. Identities must use `spec.owner.armId` outside the run-owned resource group, so run resource-group deletion cannot remove them.

The final read-only identity/role objects are not deleted from Azure by ASO when their Kubernetes objects disappear. A run-owned resource-group deletion can still remove resources contained in that group. **Role assignments on external identity scopes can survive the run.** The operator owning the privileged setup must remove those test grants explicitly after the workload is deleted; this workflow does not silently remove grants on external resources. Keep bootstrap credentials available until any required privileged cleanup is complete.

## Authorization validation still required

Before claiming a least-privilege result, run with operator-provided scoped principals and separately verify that the deployment principal is denied identity creation and role-assignment writes at every tested scope, while cluster deployment succeeds. Record the actual principals, scopes, denied Azure operations, and successful deployment evidence. No such live authorization proof is supplied by the fixture tests or this implementation.

References:

- [ASO credential scope and precedence](https://azure.github.io/azure-service-operator/guide/authentication/credential-scope/)
- [ASO reconcile policies](https://azure.github.io/azure-service-operator/guide/annotations/)
- [ASO security practices](https://azure.github.io/azure-service-operator/guide/security/)
- [Azure role assignments and scopes](https://learn.microsoft.com/en-us/azure/role-based-access-control/role-assignments)
