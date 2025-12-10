Systematically troubleshoot test failures in the ARO-CAPZ test suite.

## Instructions

Ask me:
1. Which test phase is failing (prerequisites, setup, kind-cluster, infrastructure, deployment, verification)
2. What error message or symptom I'm seeing

Then work through this diagnostic workflow:

## Diagnostic Workflow

### Step 1: Validate Prerequisites

Check core requirements:

```bash
# Tool availability
make check-prereq

# Or check specific tools
az --version
kubectl version --client
kind version
go version
```

**Common Issues**:
- Tool not installed → Install from CLAUDE.md prerequisites section
- Tool wrong version → Check version requirements
- PATH issues → Verify tools are in PATH

### Step 2: Verify Authentication

Check Azure authentication:

```bash
# Azure login status
az account show

# Correct subscription
az account list --output table
```

**Common Issues**:
- Not logged in → Run `az login`
- Wrong subscription → Run `az account set --subscription <name>`
- Expired credentials → Re-authenticate

### Step 3: Check Configuration

Verify environment variables:

```bash
# Show current config
env | grep -E "ARO|MANAGEMENT|WORKLOAD|CLUSTER|AZURE|REGION|DEPLOYMENT_ENV|USER"
```

**Common Issues**:
- Missing required vars → Set in shell or `.env`
- Incorrect values → Check against CLAUDE.md defaults
- Typos in variable names → Compare to `test/config.go`

### Step 4: Review Test Logs

Examine the actual error:

```bash
# Run specific failing test with verbose output
go test -v ./test -run Test<PhaseName> -timeout 30m
```

**Analyze**:
- Error message content
- Stack trace location
- Which specific test function failed
- Any prerequisite skip messages

### Step 5: Check Phase Dependencies

Verify previous phases completed:

**For Setup Test**:
- Prerequisites must pass
- Tools must exist

**For Kind Cluster Test**:
- Prerequisites and Setup must pass
- Repository must be cloned

**For Infrastructure Test**:
- Kind cluster must exist
- Previous phases must succeed

**For Deployment Test**:
- Infrastructure files must be generated
- Kind cluster must be running

**For Verification Test**:
- Deployment must complete
- Cluster must be accessible

### Step 6: Verify Idempotency

Test if re-running helps:

```bash
# Try running the same test again
make test-<phase>
```

**If it fails differently on re-run**:
- Test might not be idempotent
- Resources might be in inconsistent state
- Check for leftover artifacts

### Step 7: Azure Resource Validation

For deployment/infrastructure failures:

```bash
# Check resource group exists
az group show --name <RESOURCE_GROUP>

# Check for conflicting resources
az resource list --resource-group <RESOURCE_GROUP>

# Check quotas
az vm list-usage --location <REGION> --output table
```

**Common Issues**:
- Resource group doesn't exist → Create manually or check ARO_REPO workflow
- Name conflicts → Use unique cluster names
- Quota limits → Request increase or use different region
- Region unavailable → Try different region

### Step 8: Kind Cluster Validation

For kind-cluster test failures:

```bash
# List kind clusters
kind get clusters

# Check cluster health
kubectl cluster-info --context kind-<cluster-name>

# Check cluster logs
kind export logs --name <cluster-name>
```

**Common Issues**:
- Cluster already exists → Delete with `kind delete cluster --name <name>`
- Port conflicts → Check 6443 and other ports are free
- Docker issues → Restart Docker daemon

### Step 9: Repository Integration Issues

For setup test failures:

```bash
# Check if repo was cloned
ls -la /tmp/cluster-api-installer-aro

# Verify ARO_REPO_DIR is set correctly
echo $ARO_REPO_DIR

# Check repository structure
ls -la $ARO_REPO_DIR/scripts
```

**Common Issues**:
- Clone failed → Check network, GitHub access
- Wrong branch → Verify `ARO_REPO_BRANCH` setting
- Missing scripts → Repository structure changed

### Step 10: Timeout Issues

For deployment timeouts:

```bash
# Increase timeout
export DEPLOYMENT_TIMEOUT=60m
make test-deploy
```

**Common Issues**:
- Azure provisioning slow → Increase timeout
- Network issues → Check connectivity
- Resource contention → Try different time or region

## Output Format

Provide diagnosis as:

**Diagnosis for: [Test Phase] Failure**

🔍 **Root Cause**: [What's actually wrong]

🔧 **Fix**: [Specific commands or actions to resolve]

**Example**:
```bash
[Exact commands to run]
```

📝 **Prevention**: [How to avoid this in future]

**Related**:
- Reference to CLAUDE.md section if applicable
- Link to specific test file with line numbers

## Escalation

If issue persists after these steps:
1. Capture full test output: `go test -v ./test -run Test<Phase> 2>&1 | tee test-failure.log`
2. Check GitHub issues for similar problems
3. Review recent commits for breaking changes
4. Verify CLAUDE.md is followed correctly
