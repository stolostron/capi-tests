# Azure CLI Setup for Fedora 43

Fedora 43 ships with Python 3.14, which is incompatible with Azure CLI. This guide explains how to configure Azure CLI using Python 3.12.

## TL;DR - Quick Setup

Copy and run all commands at once:

```bash
# Install Python 3.12 and set as default
sudo dnf install python3.12
sudo alternatives --install /usr/bin/python3 python3 /usr/bin/python3.12 1
sudo alternatives --set python3 /usr/bin/python3.12

# Install pip and Azure CLI
python3.12 -m ensurepip --user
python3.12 -m pip install --user azure-cli

# Create az wrapper script
mkdir -p ~/.local/bin
cat > ~/.local/bin/az << 'EOF'
#!/bin/bash
exec /usr/bin/python3.12 -m azure.cli "$@"
EOF
chmod +x ~/.local/bin/az

# Add to PATH if not already present
grep -q 'HOME/.local/bin' ~/.bashrc || echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Verify installation
az --version
az login
```

---

## Problem

Fedora 43 ships with Python 3.14 as the system default. Azure CLI is incompatible with Python 3.14, causing errors like:

```
az account show
ModuleNotFoundError: No module named 'azure'
```

or

```
ImportError: Error while finding module specification for 'azure.cli'
```

This affects running `make test-all` and any Azure CLI operations in this test suite.

---

## Solution

Install Azure CLI for Python 3.12 and create a wrapper script.

### Step 1: Install Python 3.12

```bash
sudo dnf install python3.12
```

### Step 2: Set Python 3.12 as default (recommended)

```bash
sudo alternatives --install /usr/bin/python3 python3 /usr/bin/python3.12 1
sudo alternatives --set python3 /usr/bin/python3.12
```

Verify:
```bash
python3 --version
# Should show: Python 3.12.x
```

### Step 3: Install pip for Python 3.12

Python 3.12 on Fedora 43 doesn't include pip by default. Bootstrap it:

```bash
python3.12 -m ensurepip --user
```

### Step 4: Install Azure CLI

```bash
python3.12 -m pip install --user azure-cli
```

### Step 5: Create az wrapper script

Since the system `/usr/bin/az` (from RPM) uses the system Python, we need a wrapper that explicitly uses Python 3.12:

```bash
mkdir -p ~/.local/bin
cat > ~/.local/bin/az << 'EOF'
#!/bin/bash
exec /usr/bin/python3.12 -m azure.cli "$@"
EOF
chmod +x ~/.local/bin/az
```

### Step 6: Configure PATH

Ensure `~/.local/bin` is in your PATH. Add to `~/.bashrc` if not already present:

```bash
grep -q 'HOME/.local/bin' ~/.bashrc || echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Step 7: Verify and Login

```bash
# Check version
az --version

# Login to Azure
az login

# Verify account
az account show
```

---

## Running Tests

After setup, you can run the test suite:

```bash
make test-all
```

---

## Troubleshooting

### "No module named pip"

Bootstrap pip manually:
```bash
python3.12 -m ensurepip --user
```

Or download get-pip.py:
```bash
curl -sSL https://bootstrap.pypa.io/get-pip.py | python3.12 - --user
```

### "command not found: az"

Ensure `~/.local/bin` is in your PATH:
```bash
echo $PATH | grep -q '.local/bin' || echo 'PATH issue - add ~/.local/bin to PATH'
```

Reload your shell:
```bash
source ~/.bashrc
# Or open a new terminal
```

### Azure CLI RPM conflicts

If you previously installed Azure CLI via RPM, remove it first:
```bash
sudo dnf remove azure-cli
```

---

## Affected Systems

- Fedora 43 (Python 3.14 default)
- Any system where Python 3.14+ is the default

---

## Related

- [Issue #283](https://github.com/RadekCap/CAPZTests/issues/283) - Original issue tracking this problem
- [Azure CLI GitHub](https://github.com/Azure/azure-cli) - Azure CLI repository
