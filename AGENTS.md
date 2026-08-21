# AGENTS.md

Entry point for AI coding agents (and new humans) working in this repository.
This file is a **table of contents**, not a manual — it points to the
authoritative docs rather than duplicating them. Read the linked document for
detail before making changes.

> Agent-instruction files in this repo:
> [`CLAUDE.md`](CLAUDE.md) (Claude Code) and [`GEMINI.md`](GEMINI.md) (Gemini)
> carry the full, tool-specific working guidelines. `AGENTS.md` is the
> vendor-neutral index that ties them together per the
> [agents.md](https://agents.md) convention. When guidance overlaps,
> `CLAUDE.md`/`GEMINI.md` are the detailed source of truth.

## What this repo is

A Go-based end-to-end **CAPI test suite** that validates the full cluster
lifecycle (dependencies → management cluster → workload cluster deploy → verify
→ delete → cleanup) for the CAPZ/ARO and CAPA/ROSA provider paths.

| Fact | Value |
|------|-------|
| Language | Go (see [`go.mod`](go.mod) for version) |
| Tests | `./test` (phase-ordered `NN_*_test.go` files, run **sequentially**) |
| Config | Environment-variable driven — [`test/config.go`](test/config.go) |
| Shared utilities | [`test/helpers.go`](test/helpers.go) |
| Build/run entry point | [`Makefile`](Makefile) (`make test`, `make test-all`) |
| Fast local check | `make test` (dependency checks only, no cloud resources) |

## Start here

| Document | What it covers |
|----------|----------------|
| [README.md](README.md) | Overview, prerequisites, configuration reference, quick start |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | System architecture: phase model, config system, provider abstraction, execution modes |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Development workflow, branch/commit conventions, PR process |
| [CLAUDE.md](CLAUDE.md) / [GEMINI.md](GEMINI.md) | Agent working guidelines, common tasks, repo conventions |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting and secure-contribution rules |

## Working guidelines for agents

- **Tests run sequentially and are idempotent** — each phase depends on the
  previous one and skips work already done. See
  [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).
- **Never hardcode configuration.** Add config via `TestConfig` in
  [`test/config.go`](test/config.go) using `GetEnvOrDefault()`.
- **Reuse helpers.** Prefer the utilities in [`test/helpers.go`](test/helpers.go)
  over reimplementing command execution, validation, or cluster operations.
- **Follow the Go testing patterns** in
  [docs/TESTING_GUIDELINES.md](docs/TESTING_GUIDELINES.md).
- **Git workflow:** feature branch + PR, rebase (never merge) onto `main`.
  Details in [CONTRIBUTING.md](CONTRIBUTING.md).
- **Format and lint before committing:** `make fmt` and `make lint`.

## Reference documentation (`docs/`)

- [INTEGRATION.md](docs/INTEGRATION.md) — integration with `cluster-api-installer`
- [DEPENDENCIES.md](docs/DEPENDENCIES.md) — dependency management and security scanning
- [TESTING_GUIDELINES.md](docs/TESTING_GUIDELINES.md) — Go testing best practices
- [CROSS_PLATFORM.md](docs/CROSS_PLATFORM.md) — OS/shell compatibility
- [ORPHANED-RESOURCES.md](docs/ORPHANED-RESOURCES.md) — handling leftover Azure resources
- [TLDR.md](docs/TLDR.md) — quick reference
- Review records: [API_REVIEW.md](docs/API_REVIEW.md),
  [PERFORMANCE_REVIEW.md](docs/PERFORMANCE_REVIEW.md),
  [SECURITY_REVIEW.md](docs/SECURITY_REVIEW.md),
  [CI_CD_REVIEW.md](docs/CI_CD_REVIEW.md)
- [../TEST_COVERAGE.md](TEST_COVERAGE.md) — coverage analysis
- [scripts/README.md](scripts/README.md) — helper scripts
- [openshift-ci/README.md](openshift-ci/README.md) — Prow / OpenShift CI setup
