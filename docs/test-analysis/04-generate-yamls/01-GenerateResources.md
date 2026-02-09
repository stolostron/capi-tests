# Test 1: TestInfrastructure_GenerateResources

**Location:** `test/04_generate_yamls_test.go:9-97`

**Purpose:** Run the ARO infrastructure generation script and create YAML manifests.

---

## Commands Executed

| Step | Command/Action | Purpose |
|------|----------------|---------|
| 1 | Set env vars | Configure generation parameters |
| 2 | `cd <ARO_REPO_DIR>` | Change to repository directory |
| 3 | `bash aro-hcp-gen.sh <output-dir>` | Run generation script |
| 4 | Verify output files | Check each expected file exists |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ DirExists(config.RepoDir)?
      └─ No → SKIP: "Repository not cloned yet"

2. Check script exists:
   └─ FileExists(genScriptPath)?
      └─ No → FAIL: "Generation script not found"

3. Set environment variables:
   ├── DEPLOYMENT_ENV=<config.Environment>
   ├── USER=<config.CAPZUser>
   ├── WORKLOAD_CLUSTER_NAME=<config.WorkloadClusterName>
   ├── REGION=<config.Region>
   └── AZURE_SUBSCRIPTION_NAME=<config.AzureSubscriptionName> (if set)

4. Change directory:
   └─ os.Chdir(config.RepoDir)

5. Run generation script:
   └─ bash doc/aro-hcp-scripts/aro-hcp-gen.sh <output-dir-name>
      ├─ Success → Continue
      └─ Failure → FAIL with output

6. Verify output directory:
   └─ DirExists(outputDir)?
      └─ No → FAIL: "Output directory not created"

7. Verify each expected file:
   └─ For each [credentials.yaml, aro.yaml]:
      └─ FileExists(file)?
         ├─ Yes → Log file path and size
         └─ No  → FAIL: "Expected file not found"
```

---

## Environment Variables Set

```go
SetEnvVar(t, "DEPLOYMENT_ENV", config.Environment)
SetEnvVar(t, "USER", config.CAPZUser)
SetEnvVar(t, "WORKLOAD_CLUSTER_NAME", config.WorkloadClusterName)
SetEnvVar(t, "REGION", config.Region)
if config.AzureSubscriptionName != "" {
    SetEnvVar(t, "AZURE_SUBSCRIPTION_NAME", config.AzureSubscriptionName)
}
```

---

## Expected Output Files

```go
expectedFiles := []string{
    "credentials.yaml",
    "aro.yaml",
}
```

---

## Example Output

```
=== Generating infrastructure resources ===
Running infrastructure generation script: /tmp/cluster-api-installer-aro/doc/aro-hcp-scripts/aro-hcp-gen.sh stage-radek-capz-tests-cluster
✅ Infrastructure generation completed successfully
Output directory created: /tmp/cluster-api-installer-aro/stage-radek-capz-tests-cluster
  ✅ Generated file: /tmp/.../credentials.yaml (1234 bytes)
  ✅ Generated file: /tmp/.../aro.yaml (9012 bytes)
```

---

## Security Note

The full script output is NOT logged as it may contain:
- Azure subscription IDs
- Resource group names
- Other sensitive Azure configuration
