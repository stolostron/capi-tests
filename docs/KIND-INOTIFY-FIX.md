# Kind Cluster inotify Watch Limit Fix

Kind cluster creation can fail on Fedora (and other Linux distributions) due to exhausted inotify watch limits. This guide explains how to fix it.

## TL;DR - Quick Fix

Copy and run all commands at once:

```bash
# Permanent fix - increase inotify limits
echo "fs.inotify.max_user_watches=524288" | sudo tee /etc/sysctl.d/99-kind.conf
echo "fs.inotify.max_user_instances=512" | sudo tee -a /etc/sysctl.d/99-kind.conf
sudo sysctl --system

# Cleanup any failed Kind clusters
kind delete cluster --name capz-tests-stage 2>/dev/null
docker network rm kind 2>/dev/null

# Verify fix
cat /proc/sys/fs/inotify/max_user_watches
# Should show: 524288
```

---

## Problem

Kind cluster creation fails with the error:

```
ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"
```

This error message is misleading. The actual root cause can be found in the container logs:

```
inotify watch limit reached
No space left on device
Failed to add a watch for /run/systemd/ask-password: inotify watch limit reached
```

**Note**: This is NOT a disk space issue - it's a kernel limit on inotify file watches being exhausted.

---

## Cause

Fedora's default inotify limits are too low when running many applications that use file watchers:
- IDEs (VS Code, JetBrains, etc.)
- Development tools (webpack, nodemon, etc.)
- Browsers
- File sync tools (Dropbox, etc.)

Kind/Docker containers need additional inotify watches for systemd to function properly inside the container.

---

## Solution

### Temporary Fix (until reboot)

```bash
sudo sysctl fs.inotify.max_user_watches=524288
sudo sysctl fs.inotify.max_user_instances=512
```

### Permanent Fix (survives reboot)

```bash
# Create sysctl config file
echo "fs.inotify.max_user_watches=524288" | sudo tee /etc/sysctl.d/99-kind.conf
echo "fs.inotify.max_user_instances=512" | sudo tee -a /etc/sysctl.d/99-kind.conf

# Apply changes
sudo sysctl --system
```

### Cleanup Failed Clusters

After fixing the limits, clean up any failed Kind cluster artifacts:

```bash
# Delete the failed cluster
kind delete cluster --name capz-tests-stage

# Remove orphaned Docker network
docker network rm kind 2>/dev/null

# Remove any leftover containers
docker ps -a --filter "name=kind" -q | xargs -r docker rm -f
```

### Retry

```bash
make test-all
```

---

## Diagnosis Steps

If you encounter Kind cluster creation failures:

### Step 1: Create cluster with --retain flag

This keeps the container running so you can inspect logs:

```bash
kind create cluster --name test --retain
```

### Step 2: Check container logs

```bash
docker logs test-control-plane 2>&1 | grep -i "inotify\|watch\|space"
```

If you see `inotify watch limit reached`, this guide applies.

### Step 3: Check current limits

```bash
cat /proc/sys/fs/inotify/max_user_watches
cat /proc/sys/fs/inotify/max_user_instances
```

Default values are often too low (e.g., 8192 watches).

### Step 4: Cleanup test cluster

```bash
docker stop test-control-plane && docker rm test-control-plane
docker network rm kind
```

---

## Verification

After applying the fix:

```bash
# Check new limits
cat /proc/sys/fs/inotify/max_user_watches
# Should show: 524288

cat /proc/sys/fs/inotify/max_user_instances
# Should show: 512

# Test Kind cluster creation
kind create cluster --name test
kind delete cluster --name test
```

---

## Affected Systems

- Fedora 43 (and likely other recent Fedora versions)
- Ubuntu with many running applications
- Any Linux system running many file-watching applications
- Systems running Docker with Kind

---

## Related

- [Issue #285](https://github.com/RadekCap/capi-tests/issues/285) - Original issue tracking this problem
- [Kind GitHub](https://github.com/kubernetes-sigs/kind) - Kind repository
- Similar issues affect VS Code, JetBrains IDEs, and other development tools
