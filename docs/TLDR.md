# TLDR for Running Tests Locally

```bash
git clone https://github.com/RadekCap/CAPZTests.git
cd CAPZTests
git checkout dev
```

Example of a local diff for `test/config.go`:

```diff
--- a/test/config.go
+++ b/test/config.go
@@ -30,7 +30,7 @@ const (
        // DefaultCAPZUser is the default user identifier for CAPZ resources.
        // Used in ClusterNamePrefix (for resource group naming) and User field.
        // Extracted to a constant to ensure consistency across all usages.
-       DefaultCAPZUser = "rcapy"
+       DefaultCAPZUser = "rcapv"

        // DefaultDeploymentEnv is the default deployment environment identifier.
        // Used in ClusterNamePrefix and Environment field.
@@ -175,8 +175,8 @@ func NewTestConfig() *TestConfig {

        return &TestConfig{
                // Repository defaults
-               RepoURL:    GetEnvOrDefault("ARO_REPO_URL", "https://github.com/stolostron/cluster-api-installer"),
-               RepoBranch: GetEnvOrDefault("ARO_REPO_BRANCH", "installer-adobe"),
+               RepoURL:    GetEnvOrDefault("ARO_REPO_URL", "https://github.com/marek-veber/cluster-api-installer"),
+               RepoBranch: GetEnvOrDefault("ARO_REPO_BRANCH", "fix-mp-providerIDList"),
                RepoDir:    getDefaultRepoDir(),

                // Cluster defaults
```

```bash
export USE_KIND=true
make test-all
```
