# Test 2: TestSetup_VerifyRepositoryStructure

**Location:** `test/02_setup_test.go:40-62`

**Purpose:** Verify the cloned repository contains required scripts for subsequent test phases.

---

## Checks Performed

| File Path | Required By |
|-----------|-------------|
| `scripts/deploy-charts-kind-capz.sh` | Phase 3: TestKindCluster_KindClusterReady |
| `doc/aro-hcp-scripts/aro-hcp-gen.sh` | Phase 4: TestInfrastructure_GenerateResources |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ DirExists(config.RepoDir)?
      └─ No → SKIP: "Repository not cloned yet"

2. For each required script:
   │
   ├─► scripts/deploy-charts-kind-capz.sh
   │   └─ FileExists(fullPath)?
   │      ├─ Yes → Log "Found required script"
   │      └─ No  → FAIL: "Required script does not exist"
   │
   └─► doc/aro-hcp-scripts/aro-hcp-gen.sh
       └─ FileExists(fullPath)?
          ├─ Yes → Log "Found required script"
          └─ No  → FAIL: "Required script does not exist"
```

---

## Required Scripts List

```go
requiredScripts := []string{
    "scripts/deploy-charts-kind-capz.sh",
    "doc/aro-hcp-scripts/aro-hcp-gen.sh",
}
```

---

## Example Output

```
=== RUN   TestSetup_VerifyRepositoryStructure
    02_setup_test.go:59: Found required script: scripts/deploy-charts-kind-capz.sh
    02_setup_test.go:59: Found required script: doc/aro-hcp-scripts/aro-hcp-gen.sh
--- PASS: TestSetup_VerifyRepositoryStructure (0.01s)
```

---

## Dependency

This test depends on `TestSetup_CloneRepository` completing successfully. If the repository is not cloned, this test is skipped.
