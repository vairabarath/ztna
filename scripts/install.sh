#!/usr/bin/env bash
set -euo pipefail

# ZTNA Binary Installer
# Usage: curl -sL https://raw.githubusercontent.com/OWNER/REPO/main/scripts/install.sh | sudo bash -s -- -t <TYPE> -n <WORKSPACE> -u <TOKEN>

REPO_OWNER="${ZTNA_REPO_OWNER:-vairabarath}"
REPO_NAME="${ZTNA_REPO_NAME:-ztna}"
GITHUB_BASE_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}"
INSTALL_DIR="${ZTNA_INSTALL_DIR:-/usr/local/bin}"
SYSTEMD_DIR="/etc/systemd/system"

# Configuration from flags
COMPONENT_TYPE=""
WORKSPACE_ID=""
BOOTSTRAP_TOKEN=""
CONTROLLER_ADDR=""
CONTROLLER_CA_CERT=""
CONNECTOR_ADDR=""
CONNECTOR_SERVER_NAME=""
DEVICE_ID=""
DATAPLANE_LISTEN_ADDR="0.0.0.0:9443"

# Detect OS and Architecture
detect_platform() {
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$os" in
        linux)
            os="linux"
            ;;
        darwin)
            os="darwin"
            ;;
        *)
            echo "Unsupported OS: $os"
            exit 1
            ;;
    esac

    case "$arch" in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        *)
            echo "Unsupported architecture: $arch"
            exit 1
            ;;
    esac

    echo "${os}-${arch}"
}

# Get latest release version
get_latest_version() {
    curl -sL "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"tag_name": "([^"]+)".*/\1/'
}

# Global temp dir for downloads
TEMP_DIR=""

# Cleanup function
cleanup() {
    if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}

# Download binary
download_binary() {
    local component="$1"
    local version="$2"
    local platform="$3"
    local binary_name="ztna-${component}-${platform}"
    local download_url="${GITHUB_BASE_URL}/releases/download/${version}/${binary_name}"

    echo "Downloading ${component} ${version} for ${platform}..."

    TEMP_DIR=$(mktemp -d)
    
    if ! curl -fsL -o "${TEMP_DIR}/${binary_name}" "$download_url"; then
        echo "Failed to download from: $download_url"
        cleanup
        exit 1
    fi

    # Verify checksum if available
    local checksum_url="${GITHUB_BASE_URL}/releases/download/${version}/checksums.txt"
    if curl -fsL --head "$checksum_url" 2>/dev/null | grep -q "200 OK"; then
        local expected_checksum
        expected_checksum=$(curl -fsL "$checksum_url" 2>/dev/null | grep "$binary_name" | awk '{print $1}')
        if [ -n "$expected_checksum" ]; then
            local actual_checksum
            actual_checksum=$(sha256sum "${TEMP_DIR}/${binary_name}" | awk '{print $1}')
            if [ "$expected_checksum" != "$actual_checksum" ]; then
                echo "Checksum verification failed!"
                echo "Expected: $expected_checksum"
                echo "Actual:   $actual_checksum"
                cleanup
                exit 1
            fi
            echo "Checksum verified."
        fi
    fi

    chmod +x "${TEMP_DIR}/${binary_name}"
    echo "${TEMP_DIR}/${binary_name}"
}

# Install binary
install_binary() {
    local source="$1"
    local component="$2"
    local target_name="ztna-${component}"

    echo "Installing ${target_name} to ${INSTALL_DIR}..."

    if [ ! -d "$INSTALL_DIR" ]; then
        mkdir -p "$INSTALL_DIR"
    fi

    # Move binary to install directory
    if [ -w "$INSTALL_DIR" ]; then
        mv "$source" "${INSTALL_DIR}/${target_name}"
    else
        echo "Requesting sudo access to install to ${INSTALL_DIR}..."
        sudo mv "$source" "${INSTALL_DIR}/${target_name}"
    fi

    # Verify installation
    if "${INSTALL_DIR}/${target_name}" --help >/dev/null 2>&1; then
        echo "${target_name} installed successfully!"
    else
        echo "Installation may have failed. Please check ${INSTALL_DIR}/${target_name}"
    fi
}

# Create systemd service
create_systemd_service() {
    local component="$1"
    local service_name="ztna-${component}"
    local binary_path="${INSTALL_DIR}/ztna-${component}"
    local config_dir="/etc/ztna"
    local storage_dir="/var/lib/ztna/${component}"

    if [ "$EUID" -ne 0 ]; then
        echo "Skipping systemd service creation (not running as root)"
        return 0
    fi

    echo "Creating systemd service for ${component}..."

    # Create directories
    mkdir -p "$config_dir" "$storage_dir"

    # Build command line arguments based on component type
    local args=""
    args="${args} --workspace-id ${WORKSPACE_ID}"
    args="${args} --bootstrap-token ${BOOTSTRAP_TOKEN}"
    args="${args} --controller-addr ${CONTROLLER_ADDR}"
    args="${args} --storage-dir ${storage_dir}"

    if [ -n "$DEVICE_ID" ]; then
        args="${args} --device-id ${DEVICE_ID}"
    fi

    if [ -n "$CONTROLLER_CA_CERT" ] && [ -f "$CONTROLLER_CA_CERT" ]; then
        cp "$CONTROLLER_CA_CERT" "${config_dir}/controller-ca.pem"
        args="${args} --controller-ca-cert ${config_dir}/controller-ca.pem"
    fi

    if [ "$component" = "connector" ]; then
        args="${args} --dataplane-listen-addr ${DATAPLANE_LISTEN_ADDR}"
    elif [ "$component" = "agent" ]; then
        args="${args} --connector-addr ${CONNECTOR_ADDR}"
        if [ -n "$CONNECTOR_SERVER_NAME" ]; then
            args="${args} --connector-server-name ${CONNECTOR_SERVER_NAME}"
        fi
        args="${args} --dataplane-ping-interval-secs 15"
    fi

    cat > "${SYSTEMD_DIR}/${service_name}.service" <<EOF
[Unit]
Description=ZTNA ${component}
After=network.target

[Service]
Type=simple
ExecStart=${binary_path}${args}
Restart=always
RestartSec=5
User=root
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    echo "Systemd service created: ${service_name}.service"
    echo ""
    echo "To start the service:"
    echo "  sudo systemctl start ${service_name}"
    echo "  sudo systemctl enable ${service_name}"
    echo ""
    echo "To check status:"
    echo "  sudo systemctl status ${service_name}"
    echo "  sudo journalctl -u ${service_name} -f"
}

# Print usage
usage() {
    cat <<EOF
ZTNA Binary Installer

Usage: curl -sL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/scripts/install.sh | bash -s -- [OPTIONS]

Options:
    -t, --type TYPE           Component type: connector or agent (required)
    -n, --network ID          Workspace/Network ID (required)
    -u, --token TOKEN         Bootstrap token (required)
    -c, --controller ADDR     Controller address (default: https://localhost:8443)
    -d, --device-id ID        Device ID (optional, auto-generated if not provided)
    -a, --ca-cert PATH        Path to controller CA certificate (optional)
    --connector-addr ADDR     Connector address (required for agent)
    --connector-name NAME     Connector server name (optional for agent)
    --listen-addr ADDR        Dataplane listen address (default: 0.0.0.0:9443, for connector)
    -v, --version VERSION     Specific version to install (default: latest)
    -h, --help                Show this help message

Examples:
    # Install connector
    curl -sL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/scripts/install.sh | sudo bash -s -- \\
        -t connector -n my-workspace -u <BOOTSTRAP_TOKEN> -c https://controller.example.com:8443

    # Install agent
    curl -sL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/scripts/install.sh | sudo bash -s -- \\
        -t agent -n my-workspace -u <BOOTSTRAP_TOKEN> -c https://controller.example.com:8443 \\
        --connector-addr 192.168.1.60:9443

Environment Variables:
    ZTNA_REPO_OWNER    GitHub repository owner (default: your-org)
    ZTNA_REPO_NAME     GitHub repository name (default: ztna)
    ZTNA_INSTALL_DIR   Installation directory (default: /usr/local/bin)
EOF
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -t|--type)
                COMPONENT_TYPE="$2"
                shift 2
                ;;
            -n|--network)
                WORKSPACE_ID="$2"
                shift 2
                ;;
            -u|--token)
                BOOTSTRAP_TOKEN="$2"
                shift 2
                ;;
            -c|--controller)
                CONTROLLER_ADDR="$2"
                shift 2
                ;;
            -d|--device-id)
                DEVICE_ID="$2"
                shift 2
                ;;
            -a|--ca-cert)
                CONTROLLER_CA_CERT="$2"
                shift 2
                ;;
            --connector-addr)
                CONNECTOR_ADDR="$2"
                shift 2
                ;;
            --connector-name)
                CONNECTOR_SERVER_NAME="$2"
                shift 2
                ;;
            --listen-addr)
                DATAPLANE_LISTEN_ADDR="$2"
                shift 2
                ;;
            -v|--version)
                VERSION="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Validate arguments
validate_args() {
    if [ -z "$COMPONENT_TYPE" ]; then
        echo "Error: Component type is required (-t connector|agent)"
        usage
        exit 1
    fi

    if [ "$COMPONENT_TYPE" != "connector" ] && [ "$COMPONENT_TYPE" != "agent" ]; then
        echo "Error: Invalid component type: $COMPONENT_TYPE (must be 'connector' or 'agent')"
        exit 1
    fi

    if [ -z "$WORKSPACE_ID" ]; then
        echo "Error: Workspace/Network ID is required (-n)"
        usage
        exit 1
    fi

    if [ -z "$BOOTSTRAP_TOKEN" ]; then
        echo "Error: Bootstrap token is required (-u)"
        usage
        exit 1
    fi

    if [ -z "$CONTROLLER_ADDR" ]; then
        CONTROLLER_ADDR="https://localhost:8443"
        echo "Using default controller address: $CONTROLLER_ADDR"
    fi

    if [ "$COMPONENT_TYPE" = "agent" ] && [ -z "$CONNECTOR_ADDR" ]; then
        echo "Error: Connector address is required for agent (--connector-addr)"
        usage
        exit 1
    fi
}

# Main
main() {
    # Set up cleanup trap
    trap cleanup EXIT

    echo "======================================"
    echo "ZTNA Binary Installer"
    echo "======================================"
    echo ""

    parse_args "$@"
    validate_args

    local platform
    platform=$(detect_platform)

    local version
    if [ -n "${VERSION:-}" ]; then
        version="$VERSION"
    else
        version=$(get_latest_version)
        if [ -z "$version" ]; then
            echo "Failed to get latest version. Please specify with -v flag."
            exit 1
        fi
    fi

    echo "Installing ztna-${COMPONENT_TYPE} ${version} for ${platform}"
    echo "Repository: ${GITHUB_BASE_URL}"
    echo "Install directory: ${INSTALL_DIR}"
    echo ""

    local binary_path
    binary_path=$(download_binary "$COMPONENT_TYPE" "$version" "$platform")
    install_binary "$binary_path" "$COMPONENT_TYPE"

    # Binary installed successfully, clear temp dir
    TEMP_DIR=""

    echo ""
    create_systemd_service "$COMPONENT_TYPE"

    echo ""
    echo "======================================"
    echo "Installation complete!"
    echo "======================================"
    echo ""
    echo "Binary location: ${INSTALL_DIR}/ztna-${COMPONENT_TYPE}"
    echo ""

    if [ "$EUID" -ne 0 ]; then
        echo "To run manually:"
        if [ "$COMPONENT_TYPE" = "connector" ]; then
            echo "  sudo ztna-${COMPONENT_TYPE} \\"
            echo "    --workspace-id ${WORKSPACE_ID} \\"
            echo "    --bootstrap-token ${BOOTSTRAP_TOKEN} \\"
            echo "    --controller-addr ${CONTROLLER_ADDR} \\"
            echo "    --dataplane-listen-addr ${DATAPLANE_LISTEN_ADDR}"
        else
            echo "  sudo ztna-${COMPONENT_TYPE} \\"
            echo "    --workspace-id ${WORKSPACE_ID} \\"
            echo "    --bootstrap-token ${BOOTSTRAP_TOKEN} \\"
            echo "    --controller-addr ${CONTROLLER_ADDR} \\"
            echo "    --connector-addr ${CONNECTOR_ADDR} \\"
            if [ -n "$CONNECTOR_SERVER_NAME" ]; then
                echo "    --connector-server-name ${CONNECTOR_SERVER_NAME} \\"
            fi
            echo "    --dataplane-ping-interval-secs 15"
        fi
    fi
}

main "$@"
