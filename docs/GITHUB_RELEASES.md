# ZTNA GitHub Releases Setup

This guide explains how to set up GitHub Releases for distributing ZTNA binaries, similar to Twingate's installation model.

## Overview

The ZTNA project now supports Twingate-style installation:

```bash
# Install a connector
curl -sL https://raw.githubusercontent.com/vairabarath/ztna/main/scripts/install.sh | sudo bash -s -- \
    -t connector -n <WORKSPACE_ID> -u <BOOTSTRAP_TOKEN> -c <CONTROLLER_ADDR>

# Install an agent
curl -sL https://raw.githubusercontent.com/vairabarath/ztna/main/scripts/install.sh | sudo bash -s -- \
    -t agent -n <WORKSPACE_ID> -u <BOOTSTRAP_TOKEN> -c <CONTROLLER_ADDR> --connector-addr <CONNECTOR_ADDR>
```

## Prerequisites

1. A GitHub repository for your ZTNA project
2. GitHub Actions enabled for the repository
3. Docker (optional, for cross-compilation)

## Setup Steps

### 1. Update Repository Configuration

Edit the following files to set your GitHub organization/username:

#### `scripts/install.sh`
Repository owner is pre-configured:
```bash
REPO_OWNER="${ZTNA_REPO_OWNER:-vairabarath}"
REPO_NAME="${ZTNA_REPO_NAME:-ztna}"
```

#### `admin-api/src/config.ts`
Pre-configured defaults:
```typescript
githubRepoOwner: env("ZTNA_REPO_OWNER", "vairabarath"),
githubRepoName: env("ZTNA_REPO_NAME", "ztna"),
```

Or set the environment variables when running the admin-api:
```bash
export ZTNA_REPO_OWNER=vairabarath
export ZTNA_REPO_NAME=ztna
npm run dev
```

### 2. Push Code to GitHub

```bash
git init
git add .
git commit -m "Initial ZTNA setup with GitHub releases"
git remote add origin https://github.com/vairabarath/ztna.git
git push -u origin main
```

### 3. Create a Release

#### Option A: Using Git Tags (Recommended)

```bash
# Tag a new version
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

The GitHub Actions workflow will automatically:
1. Build binaries for all platforms (Linux/macOS, AMD64/ARM64)
2. Create a GitHub Release
3. Upload all binaries and checksums

#### Option B: Manual Trigger

1. Go to your GitHub repository
2. Navigate to **Actions** → **Release Binaries**
3. Click **Run workflow**
4. Enter a tag name (e.g., `v0.1.0`)
5. Click **Run workflow**

### 4. Verify the Release

After the workflow completes:

1. Go to your repository's **Releases** page
2. You should see a new release with:
   - `ztna-controller-linux-amd64`
   - `ztna-controller-linux-arm64`
   - `ztna-controller-darwin-amd64`
   - `ztna-controller-darwin-arm64`
   - `ztna-connector-linux-amd64`
   - `ztna-connector-linux-arm64`
   - `ztna-connector-darwin-amd64`
   - `ztna-connector-darwin-arm64`
   - `ztna-agent-linux-amd64`
   - `ztna-agent-linux-arm64`
   - `ztna-agent-darwin-amd64`
   - `ztna-agent-darwin-arm64`
   - `checksums.txt`

## Local Testing

### Build Binaries Locally

```bash
# Build all binaries locally
make release-all

# Build specific binaries
make release-controller-linux-amd64
make release-connector-linux-amd64
make release-agent-linux-amd64

# List built binaries
make release-list

# Clean release directory
make release-clean
```

### Test the Install Script

```bash
# Set your GitHub repo (optional, defaults are pre-configured)
export ZTNA_REPO_OWNER=vairabarath
export ZTNA_REPO_NAME=ztna

# Run the install script locally
./scripts/install.sh --help

# Test with a specific version
./scripts/install.sh -t connector -n test-workspace -u test-token -c https://localhost:8443 -v v0.1.0
```

## Admin Web Dashboard

Once the GitHub releases are set up, the admin dashboard will display Twingate-style install commands:

```
curl -sL https://raw.githubusercontent.com/vairabarath/ztna/main/scripts/install.sh | sudo bash -s -- \
    -t connector -n 'workspace-id' -u 'bootstrap-token' -c 'https://controller.example.com:8443' \
    -d 'device-id' --listen-addr '0.0.0.0:9443'
```

## LAN-Level Deployment

For LAN-level deployment:

### 1. Deploy Controller

The controller is typically deployed on a central server:

```bash
# Build and run the controller
make controller-run
```

### 2. Deploy Connectors

On each gateway/edge device in your LAN:

```bash
# Get the install command from the admin dashboard
curl -sL https://raw.githubusercontent.com/vairabarath/ztna/main/scripts/install.sh | sudo bash -s -- \
    -t connector \
    -n <WORKSPACE_ID> \
    -u <BOOTSTRAP_TOKEN> \
    -c https://<CONTROLLER_IP>:8443 \
    --listen-addr 0.0.0.0:9443
```

### 3. Deploy Agents

On each client device:

```bash
# Get the install command from the admin dashboard
curl -sL https://raw.githubusercontent.com/vairabarath/ztna/main/scripts/install.sh | sudo bash -s -- \
    -t agent \
    -n <WORKSPACE_ID> \
    -u <BOOTSTRAP_TOKEN> \
    -c https://<CONTROLLER_IP>:8443 \
    --connector-addr <CONNECTOR_IP>:9443 \
    --connector-name <CONNECTOR_SERVER_NAME>
```

## Environment Variables

### Install Script

| Variable | Description | Default |
|----------|-------------|---------|
| `ZTNA_REPO_OWNER` | GitHub repository owner | `vairabarath` |
| `ZTNA_REPO_NAME` | GitHub repository name | `ztna` |
| `ZTNA_INSTALL_DIR` | Installation directory | `/usr/local/bin` |

### Admin API

| Variable | Description | Default |
|----------|-------------|---------|
| `ZTNA_REPO_OWNER` | GitHub repository owner | `vairabarath` |
| `ZTNA_REPO_NAME` | GitHub repository name | `ztna` |

## Troubleshooting

### GitHub Actions Build Failures

**Issue**: ARM64 builds fail on GitHub Actions

**Solution**: The workflow uses `cross` for ARM64 cross-compilation. If it fails:
```bash
# Install cross locally
cargo install cross --git https://github.com/cross-rs/cross

# Build locally with cross
cd connector && cross build --release --target aarch64-unknown-linux-gnu
```

### Install Script Errors

**Issue**: "Failed to download from GitHub"

**Solution**: 
1. Verify the release exists on GitHub
2. Check that `ZTNA_REPO_OWNER` and `ZTNA_REPO_NAME` are set correctly
3. Test the download URL manually:
   ```bash
   curl -I https://github.com/vairabarath/ztna/releases/download/v0.1.0/ztna-connector-linux-amd64
   ```

### Checksum Verification Failures

**Issue**: "Checksum verification failed"

**Solution**: Regenerate checksums:
```bash
cd dist
sha256sum * > checksums.txt
```

## Security Considerations

1. **Bootstrap Tokens**: The install script contains sensitive bootstrap tokens. Ensure tokens are:
   - Short-lived (configured via `ADMIN_TOKEN_TTL_HOURS`)
   - Used only once
   - Transmitted over HTTPS

2. **Binary Verification**: The install script verifies SHA256 checksums when available. Always include `checksums.txt` in releases.

3. **CA Certificates**: For production, provide the controller's CA certificate to verify TLS connections.

## Next Steps

1. Set up your GitHub repository
2. Configure the repository owner in the config files
3. Push your code and create a release
4. Test the install commands from the admin dashboard
5. Deploy connectors and agents in your LAN

For more details on the architecture, see the main project documentation.
