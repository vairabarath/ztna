import type { AgentRecord, ConnectorRecord } from "./types.js";

function sh(input: string): string {
  return `'${input.replace(/'/g, `"'"'"`)}'`;
}

function withHeader(script: string): string {
  return [
    "#!/usr/bin/env bash",
    "set -euo pipefail",
    "",
    script.trim(),
    "",
  ].join("\n");
}

// GitHub repository configuration for binary downloads
const GITHUB_REPO_OWNER = process.env.ZTNA_REPO_OWNER || "vairabarath";
const GITHUB_REPO_NAME = process.env.ZTNA_REPO_NAME || "ztna";
const INSTALL_SCRIPT_URL = `https://raw.githubusercontent.com/${GITHUB_REPO_OWNER}/${GITHUB_REPO_NAME}/main/scripts/install.sh`;

// Base URL for admin API (for CA cert download)
const ADMIN_API_BASE = process.env.ADMIN_PUBLIC_BASE_URL || "http://localhost:8787";

/**
 * Build a Twingate-style install command for connectors
 * This generates a curl command that downloads and runs the install script
 */
export function buildConnectorInstallCommand(
  connector: ConnectorRecord,
  controllerAddr: string,
): string {
  // Extract admin API base from controller address (assume same host, port 8787)
  const controllerHost = controllerAddr.replace(/^https?:\/\//, '').split(':')[0];
  const caCertUrl = `http://${controllerHost}:8787/ca.crt`;
  
  const args = [
    `-t connector`,
    `-n ${sh(connector.workspaceId)}`,
    `-u ${sh(connector.bootstrapToken)}`,
    `-c ${sh(controllerAddr)}`,
    `-d ${sh(connector.managedDeviceId)}`,
    `--listen-addr ${sh(connector.dataplaneListenAddr)}`,
    `--ca-cert-url ${sh(caCertUrl)}`,
  ];

  return `curl -sL ${INSTALL_SCRIPT_URL} | sudo bash -s -- ${args.join(" ")}`;
}

/**
 * Build a Twingate-style install command for agents
 */
export function buildAgentInstallCommand(
  agent: AgentRecord,
  controllerAddr: string,
): string {
  // Extract admin API base from controller address (assume same host, port 8787)
  const controllerHost = controllerAddr.replace(/^https?:\/\//, '').split(':')[0];
  const caCertUrl = `http://${controllerHost}:8787/ca.crt`;
  
  const args = [
    `-t agent`,
    `-n ${sh(agent.workspaceId)}`,
    `-u ${sh(agent.bootstrapToken)}`,
    `-c ${sh(controllerAddr)}`,
    `-d ${sh(agent.managedDeviceId)}`,
    `--connector-addr ${sh(agent.connectorAddr)}`,
    `--connector-name ${sh(agent.connectorServerName)}`,
    `--ca-cert-url ${sh(caCertUrl)}`,
  ];

  return `curl -sL ${INSTALL_SCRIPT_URL} | sudo bash -s -- ${args.join(" ")}`;
}

export function buildConnectorInstallScript(
  connector: ConnectorRecord,
  controllerCaCertPem: string,
  controllerAddr: string,
): string {
  const shortId = connector.id.slice(0, 8);
  const storageDir = `/var/lib/ztna/connector-${shortId}`;

  // Use the new Twingate-style binary installation
  const body = `
# ZTNA Connector Installer
# This script installs the connector binary and configures it as a systemd service

GITHUB_REPO_OWNER="${GITHUB_REPO_OWNER}"
GITHUB_REPO_NAME="${GITHUB_REPO_NAME}"
INSTALL_DIR="/usr/local/bin"

# Detect OS and Architecture
detect_platform() {
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$os" in
        linux) os="linux" ;;
        darwin) os="darwin" ;;
        *) echo "Unsupported OS: $os"; exit 1 ;;
    esac

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) echo "Unsupported architecture: $arch"; exit 1 ;;
    esac

    echo "${os}-${arch}"
}

# Get latest release version
get_latest_version() {
    curl -sL "https://api.github.com/repos/${GITHUB_REPO_OWNER}/${GITHUB_REPO_NAME}/releases/latest" | \\
        grep '"tag_name":' | \\
        sed -E 's/.*"tag_name": "([^"]+)".*/\\1/'
}

# Download binary
download_binary() {
    local version="$1"
    local platform="$2"
    local binary_name="ztna-connector-${platform}"
    local download_url="https://github.com/${GITHUB_REPO_OWNER}/${GITHUB_REPO_NAME}/releases/download/${version}/${binary_name}"

    echo "Downloading connector ${version} for ${platform}..."
    local temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT

    if ! curl -fsL -o "${temp_dir}/ztna-connector" "$download_url"; then
        echo "Failed to download from: $download_url"
        exit 1
    fi

    chmod +x "${temp_dir}/ztna-connector"
    echo "${temp_dir}/ztna-connector"
}

# Install binary
install_binary() {
    local source="$1"
    echo "Installing ztna-connector to ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"
    mv "$source" "${INSTALL_DIR}/ztna-connector"
    echo "ztna-connector installed successfully!"
}

# Create systemd service
create_systemd_service() {
    local storageDir="${sh(storageDir)}"
    local controllerAddr="${sh(controllerAddr)}"
    
    cat > /etc/systemd/system/ztna-connector.service <<'EOF'
[Unit]
Description=ZTNA Connector
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/ztna-connector \\
  --workspace-id ${sh(connector.workspaceId)} \\
  --device-id ${sh(connector.managedDeviceId)} \\
  --bootstrap-token ${sh(connector.bootstrapToken)} \\
  --controller-addr ${controllerAddr} \\
  --storage-dir ${storageDir} \\
  --dataplane-listen-addr ${sh(connector.dataplaneListenAddr)}
Restart=always
RestartSec=5
User=root
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    echo "Systemd service created: ztna-connector.service"
}

# Main installation
main() {
    echo "======================================"
    echo "ZTNA Connector Installer"
    echo "======================================"
    echo ""

    if [ "$EUID" -ne 0 ]; then
        echo "This script must be run as root (use sudo)"
        exit 1
    fi

    local platform=$(detect_platform)
    local version=$(get_latest_version)

    if [ -z "$version" ]; then
        echo "Failed to get latest version"
        exit 1
    fi

    echo "Installing ztna-connector ${version} for ${platform}"
    echo ""

    local binary_path=$(download_binary "$version" "$platform")
    install_binary "$binary_path"
    create_systemd_service

    # Create storage directory
    mkdir -p ${sh(storageDir)}

    echo ""
    echo "======================================"
    echo "Installation complete!"
    echo "======================================"
    echo ""
    echo "To start the connector:"
    echo "  sudo systemctl start ztna-connector"
    echo "  sudo systemctl enable ztna-connector"
    echo ""
    echo "To check status:"
    echo "  sudo systemctl status ztna-connector"
    echo "  sudo journalctl -u ztna-connector -f"
}

main "$@"
`;

  return withHeader(body);
}

export function buildAgentInstallScript(
  agent: AgentRecord,
  controllerCaCertPem: string,
  controllerAddr: string,
): string {
  const shortId = agent.id.slice(0, 8);
  const storageDir = `/var/lib/ztna/agent-${shortId}`;

  const body = `
# ZTNA Agent Installer
# This script installs the agent binary and configures it as a systemd service

GITHUB_REPO_OWNER="${GITHUB_REPO_OWNER}"
GITHUB_REPO_NAME="${GITHUB_REPO_NAME}"
INSTALL_DIR="/usr/local/bin"

# Detect OS and Architecture
detect_platform() {
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$os" in
        linux) os="linux" ;;
        darwin) os="darwin" ;;
        *) echo "Unsupported OS: $os"; exit 1 ;;
    esac

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) echo "Unsupported architecture: $arch"; exit 1 ;;
    esac

    echo "${os}-${arch}"
}

# Get latest release version
get_latest_version() {
    curl -sL "https://api.github.com/repos/${GITHUB_REPO_OWNER}/${GITHUB_REPO_NAME}/releases/latest" | \\
        grep '"tag_name":' | \\
        sed -E 's/.*"tag_name": "([^"]+)".*/\\1/'
}

# Download binary
download_binary() {
    local version="$1"
    local platform="$2"
    local binary_name="ztna-agent-${platform}"
    local download_url="https://github.com/${GITHUB_REPO_OWNER}/${GITHUB_REPO_NAME}/releases/download/${version}/${binary_name}"

    echo "Downloading agent ${version} for ${platform}..."
    local temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT

    if ! curl -fsL -o "${temp_dir}/ztna-agent" "$download_url"; then
        echo "Failed to download from: $download_url"
        exit 1
    fi

    chmod +x "${temp_dir}/ztna-agent"
    echo "${temp_dir}/ztna-agent"
}

# Install binary
install_binary() {
    local source="$1"
    echo "Installing ztna-agent to ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"
    mv "$source" "${INSTALL_DIR}/ztna-agent"
    echo "ztna-agent installed successfully!"
}

# Create systemd service
create_systemd_service() {
    local storageDir="${sh(storageDir)}"
    local controllerAddr="${sh(controllerAddr)}"
    local connectorAddr="${sh(agent.connectorAddr)}"
    local connectorServerName="${sh(agent.connectorServerName)}"
    
    cat > /etc/systemd/system/ztna-agent.service <<'EOF'
[Unit]
Description=ZTNA Agent
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/ztna-agent \\
  --workspace-id ${sh(agent.workspaceId)} \\
  --device-id ${sh(agent.managedDeviceId)} \\
  --bootstrap-token ${sh(agent.bootstrapToken)} \\
  --controller-addr ${controllerAddr} \\
  --storage-dir ${storageDir} \\
  --connector-addr ${connectorAddr} \\
  --connector-server-name ${connectorServerName} \\
  --dataplane-ping-interval-secs 15
Restart=always
RestartSec=5
User=root
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    echo "Systemd service created: ztna-agent.service"
}

# Main installation
main() {
    echo "======================================"
    echo "ZTNA Agent Installer"
    echo "======================================"
    echo ""

    if [ "$EUID" -ne 0 ]; then
        echo "This script must be run as root (use sudo)"
        exit 1
    fi

    local platform=$(detect_platform)
    local version=$(get_latest_version)

    if [ -z "$version" ]; then
        echo "Failed to get latest version"
        exit 1
    fi

    echo "Installing ztna-agent ${version} for ${platform}"
    echo ""

    local binary_path=$(download_binary "$version" "$platform")
    install_binary "$binary_path"
    create_systemd_service

    # Create storage directory
    mkdir -p ${sh(storageDir)}

    echo ""
    echo "======================================"
    echo "Installation complete!"
    echo "======================================"
    echo ""
    echo "To start the agent:"
    echo "  sudo systemctl start ztna-agent"
    echo "  sudo systemctl enable ztna-agent"
    echo ""
    echo "To check status:"
    echo "  sudo systemctl status ztna-agent"
    echo "  sudo journalctl -u ztna-agent -f"
}

main "$@"
`;

  return withHeader(body);
}
