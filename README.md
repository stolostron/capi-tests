# CAPI Testing Framework

A comprehensive testing framework for Cluster API (CAPI) implementations across multiple cloud providers.

## Overview

This testing framework provides a unified approach to testing CAPI implementations including:
- **CAPZ** - Azure implementation
- **CAPA** - AWS implementation
- **CAPG** - Google Cloud implementation

## Testing Capabilities

The framework supports multiple testing levels:

### Unit Testing
Testing individual components and functions in isolation to ensure correctness at the smallest level.

### Integration Testing
Testing how multiple components work together within the CAPI ecosystem, validating interactions between controllers, providers, and Kubernetes resources.

### End-to-End Testing
Complete workflow testing that validates entire cluster lifecycle operations including provisioning, scaling, upgrading, and deprovisioning across different cloud providers.

## Technology Stack

The framework leverages multiple technologies and languages:
- **Go** - Primary programming language for test implementation
- **Ginkgo** - BDD-style testing framework for Go
- **Kuttl** - Kubernetes test tool for declarative testing
- **Shell Scripts** - Automation and helper scripts
- Additional tools and languages as needed

## Test Organization

*Details on how tests are structured, organized, and categorized will be defined here.*

## Configuration

The framework uses configuration files to manage:
- **Cloud Provider Access** - Credentials and authentication for Azure, AWS, and GCP
- **Kubernetes Clusters** - Clusters can be created dynamically by tests or provided via configuration
- **Test Parameters** - Environment-specific settings and test behavior

*Detailed configuration examples and options will be provided.*

## Getting Started

*Custom setup process for CAPI testing will be documented here, including:*
- CLI tools and development environment setup
- Initial configuration
- Running your first test

## CI/CD Integration

The framework is designed to integrate seamlessly with continuous integration pipelines.

*Details on CI/CD integration patterns, required environment variables, and pipeline examples will be documented.*

## Prerequisites

### CLI and Development Tools

Required command-line tools:
- **kubectl** - Kubernetes command-line tool (v1.25+)
- **clusterctl** - Cluster API CLI
- **kind** - Kubernetes IN Docker for local cluster testing (v0.30.0+)
- **go** - Go programming language (1.21+)
- **make** - Build automation tool
- **git** - Version control (2.40+)

Cloud provider CLIs (depending on which provider you're testing):
- **az** - Azure CLI (for CAPZ)
- **aws** - AWS CLI (for CAPA)
- **gcloud** - Google Cloud CLI (for CAPG)

### System Requirements

- Linux, macOS, or Windows with WSL2
- Minimum 8GB RAM (16GB recommended for running multiple tests)
- Docker installed and running (for containerized tests)

### Runtime Requirements
- Kubernetes cluster access (can be created by tests or provided via configuration)
- Cloud provider credentials configured according to the specific CAPI implementation being tested

## Contributing

*Guidelines for contributing to the testing framework.*

## License

*License information will be added here.*
