# Test 4: TestInfrastructure_VerifyAROClusterYAML

**Location:** `test/04_generate_yamls_test.go:161-190`

**Purpose:** Verify that `aro.yaml` (ARO cluster configuration) exists and contains valid YAML syntax.

---

## Checks Performed

| Check | Method |
|-------|--------|
| File exists | `FileExists(filePath)` |
| Valid YAML | `ValidateYAMLFile(filePath)` |
| File stats | `os.Stat(filePath)` |

---

## Detailed Flow

```
1. Build output directory path:
   └─ outputDir = <RepoDir>/<env>-<user>-<cluster>

2. Check prerequisite:
   └─ DirExists(outputDir)?
      └─ No → SKIP: "Output directory does not exist"

3. Check file exists:
   └─ FileExists(outputDir/aro.yaml)?
      └─ No → FAIL: "aro.yaml not found"

4. Validate YAML:
   └─ ValidateYAMLFile(filePath)?
      └─ Error → FAIL: "aro.yaml validation failed"

5. Get file info:
   └─ os.Stat(filePath)
      └─ Log file size
```

---

## Example Output

```
=== RUN   TestInfrastructure_VerifyAROClusterYAML
    04_generate_yamls_test.go:162: Verifying aro.yaml (ARO cluster configuration)
    04_generate_yamls_test.go:189: aro.yaml is valid YAML (size: 9012 bytes)
--- PASS: TestInfrastructure_VerifyAROClusterYAML (0.01s)
```

---

## What is aro.yaml?

The `aro.yaml` file contains the main ARO cluster definition:
- Cluster API Cluster resource
- AROCluster infrastructure reference
- AROControlPlane specification
- Network configuration
- OpenShift version

---

## Example Structure

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-aro-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks: ["10.128.0.0/14"]
    services:
      cidrBlocks: ["172.30.0.0/16"]
  controlPlaneRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: AROControlPlane
    name: my-aro-cluster
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: AROCluster
    name: my-aro-cluster
```

---

## Dependency

This test depends on `TestInfrastructure_GenerateResources` completing successfully.
