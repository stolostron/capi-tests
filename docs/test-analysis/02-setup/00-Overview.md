# Phase 2: Setup

**Make target:** `make _setup`
**Test file:** `test/02_setup_test.go`
**Timeout:** Default (2 minutes)

---

## Purpose

Clone the cluster-api-installer repository and verify it has the required structure and scripts for subsequent test phases.

---

## Test Summary

| # | Test | Purpose |
|---|------|---------|
| 1 | [01-CloneRepository](01-CloneRepository.md) | Clone cluster-api-installer repo (or verify existing) |
| 2 | [02-VerifyRepositoryStructure](02-VerifyRepositoryStructure.md) | Check required scripts exist |
| 3 | [03-ScriptPermissions](03-ScriptPermissions.md) | Ensure scripts are executable |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _setup                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 1: CloneRepository                                         │
│  ├── Check if repo directory exists                              │
│  │   ├─ Yes → Verify .git directory exists                       │
│  │   └─ No  → git clone -b <branch> <url> <dir>                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 2: VerifyRepositoryStructure                               │
│  ├── Check: scripts/deploy-charts-kind-capz.sh                   │
│  └── Check: doc/aro-hcp-scripts/aro-hcp-gen.sh                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 3: ScriptPermissions                                       │
│  └── Ensure scripts have executable bit (+x)                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ARO_REPO_URL` | `https://github.com/RadekCap/cluster-api-installer` | Repository URL |
| `ARO_REPO_BRANCH` | `ARO-ASO` | Branch to clone |
| `ARO_REPO_DIR` | `/tmp/cluster-api-installer-aro` | Local directory |

---

## Required Scripts

| Script | Used By |
|--------|---------|
| `scripts/deploy-charts-kind-capz.sh` | Phase 3 (Cluster) |
| `doc/aro-hcp-scripts/aro-hcp-gen.sh` | Phase 4 (Generate YAMLs) |
