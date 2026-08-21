# Architecture

This document describes the architecture of the CAPI test suite: how it is
structured, how the pieces fit together, and the design decisions behind them.
It is the architectural map; operational detail (every environment variable,
every Make target) lives in [../README.md](../README.md) and the agent
guideline files ([../CLAUDE.md](../CLAUDE.md), [../GEMINI.md](../GEMINI.md)).

## Purpose

The suite is a Go-based, end-to-end validation of the complete Cluster API
(CAPI) deployment workflow — from tool prerequisites through workload-cluster
verification and teardown. It currently supports two provider paths:

- **ARO** — Azure Red Hat OpenShift via CAPZ/ASO (the default)
- **ROSA** — Red Hat OpenShift on AWS via CAPA

The suite does not unit-test individual functions; it validates a real, external
deployment workflow against live cloud infrastructure.

## High-level flow

```
prerequisites ─▶ management cluster ─▶ generate YAMLs ─▶ deploy CRs
     ▲                                                        │
     │                                                        ▼
  cleanup ◀── deletion ◀──────────────── verification ◀── (cluster ready)
```

Each stage is a numbered Go test file executed in order. A later stage assumes
the resources created by earlier stages exist.

## Sequential phase model

Unlike typical Go tests, these run **sequentially**, in filename order. Each
phase is a `test/NN_*_test.go` file:

| Phase | File | Responsibility | Skip / idempotency signal |
|-------|------|----------------|---------------------------|
| 01 | `01_check_dependencies_test.go` | Tool availability, auth, config validation | Stateless — always runs |
| 02 | `02_setup_test.go` | Clone/validate `cluster-api-installer` | Directory exists → validate, skip clone |
| 03 | `03_cluster_test.go` | Management cluster (Kind or external) | `kind get clusters` / existing context |
| 04 | `04_generate_yamls_test.go` | Generate provider CR YAMLs | Expected output files already present |
| 05 | `05_deploy_crs_test.go` | Apply CRs, monitor deployment | `kubectl apply` is inherently idempotent |
| 06 | `06_verification_test.go` | Validate workload cluster health | Guards on kubeconfig + cluster phase |
| 07 | `07_deletion_test.go` | Delete the workload cluster | Skips if already deleted |
| 08 | `08_cleanup_test.go` | Validate cleanup operations | Stateless — reports current state |
| 09 | `09_teardown_test.go` | Final teardown | Resource existence checks |

### Why sequential?

- Each phase depends on state produced by the previous one.
- Phases mutate **external** state (Kind cluster, Azure/AWS resources), which
  cannot be parallelized safely within a single run.
- The goal is workflow validation, not isolated unit testing.

### Idempotency

Every phase is safe to re-run. Detection is either **file-based** (skip
generation if output exists) or **resource-based** (skip creation if the
Kubernetes/cloud resource exists), and CR application uses `kubectl apply` so it
creates-or-updates without duplicating. This lets an interrupted run resume from
where it stopped. Deployment state that must survive across phases is persisted
via the `WriteDeploymentState` / `ReadDeploymentState` helpers.

## Configuration system

All configuration is centralized in the `TestConfig` struct in
[`../test/config.go`](../test/config.go). Precedence is:

1. Environment variables (highest priority)
2. Defaults from `NewTestConfig()`

Configuration is environment-variable driven (not config files) to fit CI/CD,
keep secrets out of the repo, and follow 12-factor principles. New configuration
must be resolved through `GetEnvOrDefault()` — values are never hardcoded.

## Provider abstraction

The provider path is selected by `INFRA_PROVIDER` (`aro` | `rosa`, default
`aro`). Each provider is described by a provider-config value in
[`../test/config.go`](../test/config.go) that carries provider-specific:

- identifier and default cluster/namespace names,
- generation scripts (e.g. `scripts/aro-hcp/gen.sh` vs `scripts/rosa-hcp/gen.sh`),
- expected output files (`aro.yaml` vs `rosa.yaml`, etc.),
- controller namespaces and credential handling.

Selecting a provider shifts the defaults and the generation/deployment inputs
without changing the phase flow.

## Execution modes

The same phases run in different environments depending on configuration:

- **Kind mode** (`USE_KIND=true`) — creates a local Kind management cluster and
  installs the CAPI/provider/ASO controllers.
- **External cluster mode** (`USE_KUBECONFIG=<path>`) — runs against a
  pre-existing cluster (e.g. an MCE installation), skipping Kind creation and,
  where controllers are pre-installed, repository cloning. Includes MCE
  component detection and optional auto-enablement.

## Integration with `cluster-api-installer`

The suite consumes the [`cluster-api-installer`](https://github.com/stolostron/cluster-api-installer)
repository for deployment scripts and charts. Three sourcing strategies are
supported (details in [INTEGRATION.md](INTEGRATION.md)):

1. **Git submodule** (recommended for development)
2. **Automatic clone** (default for CI/CD — cloned to a temp dir)
3. **Existing local clone** (via `ARO_REPO_DIR`)

## Shared helpers

[`../test/helpers.go`](../test/helpers.go) provides the reusable layer used by
every phase — command execution, path/config validation, YAML extraction,
cluster operations, cloud-credential setup, error detection/formatting,
controller-log analysis, deployment-state persistence, and MCE integration.
Tests always use these helpers rather than reimplementing the behavior, which
keeps error handling, logging, and cleanup consistent across phases.

## Repository layout

```
.
├── test/                 # Phase-ordered tests + config.go + helpers.go + support code
├── scripts/              # Cleanup, monitoring, and reporting shell scripts
├── openshift-ci/         # Prow / OpenShift CI configuration and step registry
├── .github/workflows/    # GitHub Actions: management/workload cluster runs + security scans
├── docs/                 # Architecture, integration, testing, and review documentation
├── Makefile              # Primary build/run entry point (test, cleanup, tooling targets)
├── AGENTS.md             # Index of context docs for AI agents and new contributors
├── CLAUDE.md / GEMINI.md # Agent-specific working guidelines
├── README.md             # Overview, configuration reference, quick start
└── CONTRIBUTING.md       # Development workflow and conventions
```

## Test execution entry points

- `make test` — fast dependency checks only (no cloud resources).
- `make test-all` — the full phase sequence.
- Individual phases are exposed as internal Make targets and can be run directly
  with `go test -run TestPhase...`. See [../README.md](../README.md) and the
  [Makefile](../Makefile) for the complete target list.

## CI/CD

GitHub Actions run the fast dependency checks and a battery of security scans
(CodeQL, gosec, govulncheck, nancy, trivy, scorecard, fuzz) on every change,
plus scheduled management/workload cluster runs for both providers. Prow /
OpenShift CI configuration lives under [../openshift-ci/](../openshift-ci/). See
[CI_CD_REVIEW.md](CI_CD_REVIEW.md) for a review of the pipeline.

## Related documentation

- [../README.md](../README.md) — configuration reference and usage
- [../AGENTS.md](../AGENTS.md) — documentation index
- [INTEGRATION.md](INTEGRATION.md) — `cluster-api-installer` integration
- [TESTING_GUIDELINES.md](TESTING_GUIDELINES.md) — Go testing best practices
- [DEPENDENCIES.md](DEPENDENCIES.md) — dependency management
- [CROSS_PLATFORM.md](CROSS_PLATFORM.md) — OS/shell compatibility
