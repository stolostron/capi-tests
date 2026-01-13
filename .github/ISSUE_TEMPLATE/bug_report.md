---
name: Bug Report
about: Report a bug in the ARO-CAPZ test suite
title: '[BUG] '
labels: bug
assignees: ''
---

## Bug Description

A clear description of the bug.

## Test Phase

Which test phase failed?

- [ ] Check Dependencies (01)
- [ ] Setup (02)
- [ ] Kind Cluster (03)
- [ ] YAML Generation (04)
- [ ] CR Deployment (05)
- [ ] Verification (06)
- [ ] Other/Unknown

## Environment

### Local Setup

- **OS**: <!-- e.g., macOS 14.0, Ubuntu 22.04, Fedora 43 -->
- **Go version**: <!-- output of `go version` -->
- **Docker/Podman**: <!-- version -->

### Azure Configuration

- **Region**: <!-- e.g., uksouth, westus2 -->
- **OpenShift Version**: <!-- e.g., 4.21 -->
- **DEPLOYMENT_ENV**: <!-- e.g., stage, prod -->

### Cluster State

- **Kind cluster exists**: <!-- yes/no -->
- **CAPI controllers running**: <!-- yes/no, or unknown -->

## Steps to Reproduce

1. Run `make ...`
2. ...
3. See error

## Expected Behavior

What should happen.

## Actual Behavior

What actually happened.

## Error Output

```
Paste relevant error output here
```

## Troubleshooting Attempted

- [ ] Ran `make clean` and retried
- [ ] Verified Azure CLI authenticated (`az account show`)
- [ ] Checked prerequisites (`make check-prereq`)
- [ ] Reviewed logs in `results/latest/`

## Additional Context

Any other relevant information, screenshots, or logs.
