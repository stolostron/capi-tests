# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly:

1. **Do NOT** open a public GitHub issue for security vulnerabilities
2. Email the maintainers directly or use GitHub's private vulnerability reporting
3. Include the following information:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 7 days
- **Fix/mitigation**: Based on severity

## Automated Security Scanning

This repository uses automated security scanning on every push and on a daily schedule:

| Scanner | Purpose | Workflow |
|---------|---------|----------|
| [gosec](https://github.com/securego/gosec) | Go source code security analysis | `security-gosec.yml` |
| [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Go vulnerability database check | `security-govulncheck.yml` |
| [Trivy](https://github.com/aquasecurity/trivy) | Comprehensive vulnerability scanner | `security-trivy.yml` |
| [nancy](https://github.com/sonatype-nexus-community/nancy) | Dependency vulnerability check | `security-nancy.yml` |

Security findings automatically create GitHub issues with remediation guidance.

## Supported Versions

| Version | Supported |
|---------|-----------|
| main branch | Yes |
| Feature branches | Development only |

## Security Best Practices for Contributors

When contributing to this project:

- **Never commit secrets or credentials** (API keys, passwords, tokens)
- **Use environment variables** for sensitive configuration
- **Review security scan results** before merging PRs
- **Follow RFC 1123 naming** for Kubernetes resources
- **Clean up temporary files** (kubeconfig, generated YAMLs)

### Sensitive Files to Never Commit

- `.env` files with credentials
- `kubeconfig` files
- Azure service principal secrets
- SSH keys or certificates
- Any file containing `AZURE_CLIENT_SECRET`

## Known Security Considerations

This test suite interacts with Azure infrastructure and requires:

- Azure CLI authentication or service principal credentials
- Access to create/delete Azure resources
- Kubernetes cluster admin privileges

Ensure you:
- Use dedicated test subscriptions when possible
- Clean up resources after testing (`make clean`)
- Rotate service principal credentials regularly
- Use least-privilege access for CI/CD
