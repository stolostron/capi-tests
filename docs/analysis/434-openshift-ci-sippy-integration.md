# Analysis: OpenShift CI and Sippy Integration

**Issue:** [#434 - Integrate with OpenShift CI and Sippy reporting](https://github.com/RadekCap/CAPZTests/issues/434)

**Date:** 2025-01-28

**Status:** Analysis Complete

---

## Executive Summary

This document outlines the integration of the ARO-CAPZ test suite with OpenShift CI (Prow) and Sippy analytics. The goal is to run tests automatically in OpenShift CI and have results visible in Sippy dashboards for tracking test reliability and regressions.

**Key Finding:** The test suite already produces JUnit XML output (required by Sippy). The remaining work is CI infrastructure configuration and onboarding.

---

## Current State

### What's Already Implemented

| Component | Status | Details |
|-----------|--------|---------|
| JUnit XML output | Done | All test phases produce `junit-*.xml` files via `gotestsum` |
| Test phases | Done | 8 sequential phases with proper exit codes |
| MCE mode | In Progress | `USE_KUBECONFIG` support (issue #433) |
| Azure credential handling | Partial | Works via env vars, needs CI secrets integration |

### What's Needed

| Component | Status | Details |
|-----------|--------|---------|
| OpenShift CI job configs | Not Started | Prow configuration for presubmit/periodic jobs |
| Sippy integration | Not Started | Ensure test results flow to Sippy dashboards |
| CI secrets | Not Started | Azure credentials as Prow secrets |
| Documentation | Not Started | CI/CD runbook and troubleshooting guide |

---

## Technical Requirements

### OpenShift CI (Prow)

**Job Types Required:**

| Job Type | Trigger | Purpose |
|----------|---------|---------|
| Presubmit | On every PR | Validate changes don't break tests |
| Periodic | Scheduled (e.g., nightly) | Continuous validation, Sippy data collection |

**Configuration Files:**
- Prow job definitions (YAML)
- CI Operator config
- Secret references for Azure credentials

**Cluster Provisioning:**
- Tests will run in MCE mode (`USE_KUBECONFIG`)
- CI provides pre-configured cluster with CAPI/CAPZ/ASO controllers
- Tests validate controllers and deploy workload cluster

### Sippy Integration

**Data Flow:**
```
Test Run → JUnit XML → Prow Artifacts (GCS) → Sippy Ingestion → Dashboard
```

**Requirements:**
- JUnit XML files in correct artifact path (already implemented)
- Job registered in Sippy configuration
- Proper test naming for Sippy categorization

---

## Implementation Approach

### Phase 1: Research & Preparation

- Study existing OpenShift CI jobs as templates
- Identify similar projects for reference patterns
- Understand Prow configuration structure
- Map Azure credential requirements to CI secrets

### Phase 2: OpenShift CI Onboarding

- Create Prow job configuration files
- Configure CI Operator settings
- Set up Azure credentials as CI secrets
- Implement presubmit job (PR validation)
- Implement periodic job (scheduled runs)

### Phase 3: Sippy Integration

- Register jobs with Sippy
- Validate JUnit XML format compatibility
- Verify test results appear in Sippy dashboards
- Configure alerting/notifications if needed

### Phase 4: Validation & Documentation

- End-to-end testing of CI pipeline
- Create troubleshooting documentation
- Knowledge transfer and runbook

---

## Rough Effort Estimation

> **Disclaimer:** These estimates are rough approximations based on limited knowledge of OpenShift CI/Prow internals. Actual effort may vary significantly based on:
> - OpenShift CI onboarding complexity
> - Access and permissions requirements
> - Review cycles and dependencies on other teams
> - Unexpected technical challenges

### Estimation Summary

| Phase | Estimated Effort | Calendar Time | Confidence |
|-------|------------------|---------------|------------|
| Research & Preparation | 2-3 days | 1 week | Medium |
| OpenShift CI Onboarding | 3-5 days | 2-3 weeks | Low |
| Sippy Integration | 1-2 days | 1-2 weeks | Medium |
| Validation & Documentation | 2-3 days | 1 week | Medium |
| **Total** | **8-13 days effort** | **5-7 weeks calendar** | **Low** |

### Phase Breakdown

#### Phase 1: Research & Preparation (2-3 days effort, 1 week calendar)

| Task | Effort | Notes |
|------|--------|-------|
| Study OpenShift CI documentation | 0.5 day | Understand Prow, CI Operator concepts |
| Analyze existing job configurations | 1 day | Find similar projects as templates |
| Map credential requirements | 0.5 day | Azure secrets, service accounts |
| Create implementation plan | 0.5 day | Detailed tasks based on findings |

**Risks:**
- Documentation may be sparse or outdated
- May need to contact CI team for guidance

#### Phase 2: OpenShift CI Onboarding (3-5 days effort, 2-3 weeks calendar)

| Task | Effort | Notes |
|------|--------|-------|
| Create initial Prow configuration | 1 day | Based on templates |
| Set up CI secrets for Azure | 0.5 day | May require approvals |
| Implement presubmit job | 1 day | PR validation |
| Implement periodic job | 0.5 day | Scheduled runs |
| Iterate on review feedback | 1-2 days | PR reviews, fixes |

**Risks:**
- Onboarding process may have bureaucratic overhead
- Secret management may require security review
- Review cycles can extend calendar time significantly
- "Won't be as straightforward" warning suggests hidden complexity

**Calendar time note:** The 2-3 week estimate accounts for:
- PR review turnaround times
- Potential back-and-forth with CI team
- Access/permissions requests

#### Phase 3: Sippy Integration (1-2 days effort, 1-2 weeks calendar)

| Task | Effort | Notes |
|------|--------|-------|
| Register jobs with Sippy | 0.5 day | Configuration update |
| Validate data flow | 0.5 day | Verify JUnit → GCS → Sippy |
| Tune test categorization | 0.5 day | Proper naming/tagging |

**Risks:**
- May need Sippy team involvement
- Data may take time to appear in dashboards

#### Phase 4: Validation & Documentation (2-3 days effort, 1 week calendar)

| Task | Effort | Notes |
|------|--------|-------|
| End-to-end testing | 1 day | Full pipeline validation |
| Create runbook | 0.5 day | Operations guide |
| Troubleshooting guide | 0.5 day | Common issues and fixes |
| Knowledge transfer | 0.5 day | Team documentation |

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| OpenShift CI complexity higher than expected | High | High | Allocate buffer time, engage CI team early |
| Review cycles extend timeline | High | Medium | Start reviews early, respond quickly |
| Azure credential setup blocked by security | Medium | High | Engage security team early |
| Sippy configuration requires team involvement | Medium | Low | Prepare requirements document upfront |
| MCE mode (#433) not ready | Low | High | Prioritize #433 completion first |

---

## Dependencies

### Blocking Dependencies

| Dependency | Status | Impact |
|------------|--------|--------|
| Issue #433 (MCE mode) | In Progress | Required for CI cluster integration |
| Azure service principal for CI | Not Started | Required for tests to authenticate |
| OpenShift CI repo access | Unknown | Required to submit Prow configs |

### Non-Blocking Dependencies

| Dependency | Status | Impact |
|------------|--------|--------|
| Issue #435 (ASO credentials) | Waiting | Nice to have for validation |
| Issue #437 (K8S namespace) | Waiting | Clarification only |

---

## Recommendations for Management

### Timeline Guidance

**Best Case:** 5 weeks calendar time
- Smooth onboarding, quick reviews, no blockers

**Expected Case:** 6-7 weeks calendar time
- Normal review cycles, minor issues resolved quickly

**Worst Case:** 10+ weeks calendar time
- Significant onboarding hurdles, security reviews, blocked dependencies

### Key Messages

1. **JUnit XML output is already done** - the core technical work for Sippy compatibility is complete

2. **Remaining work is infrastructure/configuration** - not development work, but CI onboarding which is process-dependent

3. **Timeline is review-dependent** - much of the calendar time is waiting for PR reviews and approvals, not active development

4. **External dependencies exist** - OpenShift CI team involvement, security approvals for credentials

5. **Uncertainty is high** - "won't be straightforward" warning from team suggests expect surprises

### Suggested Approach

1. **Start with research phase** - low cost, high learning value
2. **Engage CI team early** - understand actual process and requirements
3. **Provide range estimates** - given uncertainty, communicate 5-10 week range
4. **Identify blockers early** - credentials, access, approvals

---

## Next Steps

1. [ ] Complete issue #433 (MCE mode) - blocking dependency
2. [ ] Research OpenShift CI documentation and find template jobs
3. [ ] Identify point of contact for OpenShift CI onboarding
4. [ ] Request Azure service principal for CI environment
5. [ ] Create detailed implementation tasks after research phase

---

## Related Issues

- #433 - External kubeconfig support (blocking)
- #435 - ASO credentials validation
- #436 - Deployment modes documentation
- #437 - K8S namespace clarification

---

## Appendix: Reference Resources

### OpenShift CI

- [OpenShift CI Documentation](https://docs.ci.openshift.org/)
- [CI Operator Documentation](https://docs.ci.openshift.org/docs/architecture/ci-operator/)
- [Prow Job Configuration](https://docs.ci.openshift.org/docs/how-tos/contributing-openshift-release/)

### Sippy

- [Sippy GitHub Repository](https://github.com/openshift/sippy)
- [Sippy Dashboard](https://sippy.dptools.openshift.org/)

### This Project

- Test output: JUnit XML files in `results/` directory
- Makefile targets: `make test-all` runs full suite with JUnit output
- MCE mode: `USE_KUBECONFIG=<path>` (issue #433)
