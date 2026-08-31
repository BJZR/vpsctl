#!/usr/bin/env bash
#
# vpsctl Installer
# Installs and configures the vpsctl VPS management panel.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/vpsctl/vpsctl/main/scripts/install.sh | sudo bash
#   sudo bash install.sh              # Interactive install
#   sudo bash install.sh --uninstall  # Remove vpsctl
#
# Supported OS: Ubuntu 20.04+, Debian 11+, CentOS 8+, AlmaLinux 8+
# Architecture: amd64, arm64
#
set -euo pipefail

# ============================================================================
# Constants
# ============================================================================
readonly VPSCTL_VERSION="${VPSCTL_VERSION:-latest}"
readonly INSTALL_DIR="/usr/local/bin"
readonly CONFIG_DIR="/etc/vpsctl"
readonly DATA_DIR="/var/lib/vpsctl"
readonly CERT_DIR="/etc/vpsctl/certs"
readonly LOG_DIR="/var/log/vpsctl"
readonly SERVICE_NAME="vpsctl"
readonly SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
readonly DEFAULT_PORT=8443
readonly DEFAULT_USER="admin"
readonly GITHUB_REPO="vpsctl/vpsctl"
readonly DOWNLOAD_BASE="https://github.com/${GITHUB_REPO}/releases/download"

# ============================================================================
# Colors & Formatting
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ============================================================================
# Utility Functions
# ============================================================================

# Print a success message
info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

# Print a warning message
warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

# Print an error message and exit
error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
    exit 1
}

# Print a step header
step() {
    echo ""
    echo -e "${CYAN}▸${NC} ${BOLD}$*${NC}"
}

# Print the vpsctl banner
print_banner() {
    echo -e "${BLUE}"
    cat << 'BANNER'

  __   __  ___   __  __  ____  ____
  \ \ / / / _ \  \ \/ / / ___||  _ \
   \ V / | | | |  \  /  \___ \| | | |
   | |  | |_| |  /  \   ___) | |_| |
   |_|   \___/  /_/\_\ |____/|____/

  VPS Control Panel Installer

BANNER
    echo -e "${NC}"
}

# Generate a random string of specified length
generate_random() {
    local length=${1:-32}
    if command -v openssl &>/dev/null; then
        openssl rand -hex "$((length / 2))" 2>/dev/null || head -c "$length" /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c "$length"
    else
        head -c "$length" /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c "$length"
    fi
}

# Generate a random password
generate_password() {
    generate_random 16
}

# Check if a command exists
command_exists() {
    command -v "$1" &>/dev/null
}

# Detect the system architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            error "Unsupported architecture: $arch"
            ;;
    esac
}

# Detect the operating system
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        case "$ID" in
            ubuntu|debian|linuxmint)
                echo "debian"
                ;;
            centos|rhel|rocky|almalinux|ol|amzn)
                echo "rhel"
                ;;
            fedora)
                echo "fedora"
                ;;
            *)
                if [[ "${ID_LIKE:-}" == *"debian"* ]]; then
                    echo "debian"
                elif [[ "${ID_LIKE:-}" == *"rhel"* ]] || [[ "${ID_LIKE:-}" == *"centos"* ]]; then
                    echo "rhel"
                else
                    error "Unsupported OS: $ID ($VERSION_ID)"
                fi
                ;;
        esac
    elif [[ -f /etc/redhat-release ]]; then
        echo "rhel"
    elif [[ -f /etc/debian_version ]]; then
        echo "debian"
    else
        error "Unable to detect operating system"
    fi
}

# ============================================================================
# Root Check
# ============================================================================

check_root() {
    step "Checking permissions"
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root. Use: sudo bash $0"
    fi
    info "Running as root"
}

# ============================================================================
# System Detection
# ============================================================================

detect_system() {
    step "Detecting system"
    local os arch
    os=$(detect_os)
    arch=$(detect_arch)

    echo -e "  OS:         ${BOLD}${os}${NC}"
    echo -e "  Architecture: ${BOLD}${arch}${NC}"
    echo -e "  Kernel:     $(uname -r)"

    export VPSCTL_OS="$os"
    export VPSCTL_ARCH="$arch"

    info "System: ${os}/${arch}"
}

# ============================================================================
# Package Installation
# ============================================================================

install_dependencies() {
    step "Installing dependencies"
    local packages=(curl wget tar gzip openssl ca-certificates)

    if command -v apt-get &>/dev/null; then
        info "Using apt package manager"
        apt-get update -qq
        apt-get install -y -qq "${packages[@]}" >/dev/null 2>&1
    elif command -v yum &>/dev/null; then
        info "Using yum package manager"
        yum install -y -q "${packages[@]}" >/dev/null 2>&1
    elif command -v dnf &>/dev/null; then
        info "Using dnf package manager"
        dnf install -y -q "${packages[@]}" >/dev/null 2>&1
    else
        warn "No supported package manager found. Ensure dependencies are installed manually."
    fi

    info "Dependencies installed"
}

# ============================================================================
# Docker Installation
# ============================================================================

check_docker() {
    step "Checking Docker"
    if command_exists docker; then
        local docker_version
        docker_version=$(docker --version 2>/dev/null | awk '{print $3}' | tr -d ',')
        info "Docker is installed (version ${docker_version})"

        if docker info &>/dev/null; then
            info "Docker daemon is running"
        else
            warn "Docker daemon is not running. Starting Docker..."
            systemctl start docker || error "Failed to start Docker"
            systemctl enable docker || true
        fi
    else
        warn "Docker is not installed. Installing Docker..."
        install_docker
    fi
}

install_docker() {
    step "Installing Docker"
    info "Installing Docker using the official convenience script..."

    if command -v curl &>/dev/null; then
        curl -fsSL https://get.docker.com | sh 2>/dev/null
    else
        error "curl is required to install Docker"
    fi

    systemctl start docker
    systemctl enable docker

    info "Docker installed successfully"
}

# ============================================================================
# System User
# ============================================================================

create_system_user() {
    step "Creating system user"
    if id "vpsctl" &>/dev/null; then
        info "User 'vpsctl' already exists"
    else
        useradd --system --no-create-home --shell /usr/sbin/nologin vpsctl 2>/dev/null || \
            adduser --system --no-create-home --shell /usr/sbin/nologin vpsctl 2>/dev/null || \
            error "Failed to create system user 'vpsctl'"

        info "User 'vpsctl' created"
    fi

    # Add vpsctl user to docker group if Docker is available
    if command_exists docker && getent group docker &>/dev/null; then
        usermod -aG docker vpsctl 2>/dev/null || true
        info "User 'vpsctl' added to docker group"
    fi
}

# ============================================================================
# Binary Installation
# ============================================================================

install_binary() {
    step "Installing vpsctl binary"

    local version="$VPSCTL_VERSION"
    local arch="$VPSCTL_ARCH"
    local binary_name="vpsctl_linux_${arch}"
    local download_url

    if [[ "$version" == "latest" ]]; then
        download_url="https://github.com/${GITHUB_REPO}/releases/latest/download/${binary_name}.tar.gz"
    else
        download_url="${DOWNLOAD_BASE}/v${version}/${binary_name}.tar.gz"
    fi

    info "Downloading vpsctl from: ${download_url}"

    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    if command -v curl &>/dev/null; then
        curl -fsSL -o "${tmp_dir}/vpsctl.tar.gz" "$download_url" || {
            warn "Download failed. Attempting to build from source..."
            build_from_source "$tmp_dir"
            return
        }
    elif command -v wget &>/dev/null; then
        wget -q -O "${tmp_dir}/vpsctl.tar.gz" "$download_url" || {
            warn "Download failed. Attempting to build from source..."
            build_from_source "$tmp_dir"
            return
        }
    else
        error "Neither curl nor wget is available"
    fi

    # Extract the binary
    tar -xzf "${tmp_dir}/vpsctl.tar.gz" -C "$tmp_dir" 2>/dev/null || \
        tar -xf "${tmp_dir}/vpsctl.tar.gz" -C "$tmp_dir" 2>/dev/null

    # Find the binary in the extracted files
    local binary_path
    binary_path=$(find "$tmp_dir" -name "vpsctl" -type f -executable 2>/dev/null | head -1)
    if [[ -z "$binary_path" ]]; then
        binary_path=$(find "$tmp_dir" -name "vpsctl" -type f 2>/dev/null | head -1)
    fi

    if [[ -z "$binary_path" ]]; then
        error "Could not find vpsctl binary in the downloaded archive"
    fi

    # Install the binary
    chmod +x "$binary_path"
    mv "$binary_path" "${INSTALL_DIR}/vpsctl"

    info "Binary installed to ${INSTALL_DIR}/vpsctl"
}

build_from_source() {
    local tmp_dir=${1:-$(mktemp -d)}

    step "Building from source"

    if ! command -v go &>/dev/null; then
        warn "Go is not installed. Attempting to install Go..."
        install_go "$tmp_dir"
    fi

    local go_version
    go_version=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
    info "Go version: ${go_version:-unknown}"

    # Clone the repository
    info "Cloning repository..."
    git clone --depth 1 "https://github.com/${GITHUB_REPO}.git" "${tmp_dir}/vpsctl-src" 2>/dev/null || \
        error "Failed to clone repository"

    cd "${tmp_dir}/vpsctl-src"

    # Build the binary
    info "Building vpsctl..."
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=dev" -o "${tmp_dir}/vpsctl" ./cmd/vpsctl || \
        CGO_ENABLED=0 go build -ldflags="-s -w" -o "${tmp_dir}/vpsctl" . || \
        error "Build failed"

    # Install the binary
    chmod +x "${tmp_dir}/vpsctl"
    mv "${tmp_dir}/vpsctl" "${INSTALL_DIR}/vpsctl"

    info "Binary built and installed to ${INSTALL_DIR}/vpsctl"
    cd -
}

install_go() {
    local tmp_dir=${1:-$(mktemp -d)}
    local go_version="1.22.4"
    local arch
    arch=$(uname -m)

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
    esac

    info "Downloading Go ${go_version}..."
    curl -fsSL "https://go.dev/dl/go${go_version}.linux-${arch}.tar.gz" -o "${tmp_dir}/go.tar.gz" || \
        error "Failed to download Go"

    # Remove any existing Go installation and install fresh
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "${tmp_dir}/go.tar.gz"

    export PATH="/usr/local/go/bin:$PATH"
    export GOPATH="/root/go"
    export PATH="$GOPATH/bin:$PATH"

    info "Go installed: $(go version 2>/dev/null)"
}

# ============================================================================
# Directories
# ============================================================================

create_directories() {
    step "Creating directories"

    local dirs=(
        "$CONFIG_DIR"
        "$CONFIG_DIR/certs"
        "$DATA_DIR"
        "$LOG_DIR"
        "$DATA_DIR/backups"
    )

    for dir in "${dirs[@]}"; do
        mkdir -p "$dir"
        info "Created: ${dir}"
    done

    # Set ownership
    chown -R vpsctl:vpsctl "$DATA_DIR" 2>/dev/null || true
    chown -R vpsctl:vpsctl "$LOG_DIR" 2>/dev/null || true

    info "Directories created"
}

# ============================================================================
# TLS Certificates
# ============================================================================

generate_tls_cert() {
    step "Generating TLS certificate"

    local cert_file="${CERT_DIR}/server.crt"
    local key_file="${CERT_DIR}/server.key"

    # Check if certificates already exist
    if [[ -f "$cert_file" ]] && [[ -f "$key_file" ]]; then
        info "TLS certificates already exist. Skipping generation."
        return
    fi

    # Get the server's IP address for the certificate
    local server_ip
    server_ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")

    info "Generating self-signed TLS certificate..."

    openssl req -x509 -nodes -days 365 \
        -newkey rsa:2048 \
        -keyout "$key_file" \
        -out "$cert_file" \
        -subj "/C=US/ST=State/L=City/O=vpsctl/CN=${server_ip}" \
        -addext "subjectAltName=IP:${server_ip},DNS:localhost,IP:127.0.0.1" \
        2>/dev/null || \
    openssl req -x509 -nodes -days 365 \
        -newkey rsa:2048 \
        -keyout "$key_file" \
        -out "$cert_file" \
        -subj "/C=US/ST=State/L=City/O=vpsctl/CN=${server_ip}" \
        2>/dev/null || \
        error "Failed to generate TLS certificate"

    # Set permissions
    chmod 600 "$key_file"
    chmod 644 "$cert_file"
    chown vpsctl:vpsctl "$key_file" "$cert_file" 2>/dev/null || true

    info "TLS certificate generated: ${cert_file}"
}

# ============================================================================
# Configuration
# ============================================================================

generate_config() {
    step "Generating configuration"

    local config_file="${CONFIG_DIR}/config.yaml"

    # Check if config already exists
    if [[ -f "$config_file" ]]; then
        warn "Configuration file already exists at ${config_file}"
        warn "Skipping configuration generation to preserve existing config."
        return
    fi

    local jwt_secret
    jwt_secret=$(generate_random 64)

    local default_password
    default_password=$(generate_password)

    cat > "$config_file" << YAML
# vpsctl Configuration
# Documentation: https://github.com/vpsctl/vpsctl

server:
  host: "0.0.0.0"
  port: ${DEFAULT_PORT}
  tls:
    enabled: true
    cert: "${CERT_DIR}/server.crt"
    key: "${CERT_DIR}/server.key"

auth:
  jwt_secret: "${jwt_secret}"
  session_timeout: "24h"
  max_login_attempts: 5
  lockout_duration: "15m"

database:
  driver: "sqlite"
  path: "${DATA_DIR}/data.db"

docker:
  socket: "/var/run/docker.sock"

logging:
  level: "info"
  file: "${LOG_DIR}/vpsctl.log"
  max_size: "100MB"
  max_backups: 5

backup:
  enabled: true
  path: "${DATA_DIR}/backups"
  schedule: "0 2 * * *"
  retention: "30d"
YAML

    # Set permissions
    chmod 640 "$config_file"
    chown vpsctl:vpsctl "$config_file" 2>/dev/null || true

    # Save the default password for display
    echo "$default_password" > "${CONFIG_DIR}/.default_password"
    chmod 600 "${CONFIG_DIR}/.default_password"
    chown vpsctl:vpsctl "${CONFIG_DIR}/.default_password" 2>/dev/null || true

    info "Configuration generated: ${config_file}"
    info "Default password saved to: ${CONFIG_DIR}/.default_password"
}

# ============================================================================
# Systemd Service
# ============================================================================

create_systemd_service() {
    step "Creating systemd service"

    cat > "$SERVICE_FILE" << SERVICE
[Unit]
Description=vpsctl - VPS Control Panel
Documentation=https://github.com/vpsctl/vpsctl
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
User=vpsctl
Group=vpsctl
ExecStart=${INSTALL_DIR}/vpsctl server
Restart=on-failure
RestartSec=5
StartLimitBurst=5
StartLimitIntervalSec=60

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${LOG_DIR} /var/run/docker.sock
PrivateTmp=true
ProtectKernelTunables=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictNamespaces=true
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

# Environment
Environment="VPSCTL_CONFIG=${CONFIG_DIR}/config.yaml"
Environment="VPSCTL_DATA_DIR=${DATA_DIR}"
EnvironmentFile=-${CONFIG_DIR}/env

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=vpsctl

[Install]
WantedBy=multi-user.target
SERVICE

    # Reload systemd
    systemctl daemon-reload

    info "Systemd service created: ${SERVICE_FILE}"
}

# ============================================================================
# Enable & Start Service
# ============================================================================

enable_and_start() {
    step "Enabling and starting vpsctl service"

    systemctl enable "$SERVICE_NAME" 2>/dev/null || true

    systemctl start "$SERVICE_NAME" 2>/dev/null || {
        warn "Service failed to start. Checking status..."
        systemctl status "$SERVICE_NAME" --no-pager 2>/dev/null || true
        warn "You may need to start the service manually: systemctl start vpsctl"
        return
    }

    # Wait a moment for the service to start
    sleep 2

    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "vpsctl service is running"
    else
        warn "vpsctl service may not be running. Check: systemctl status vpsctl"
    fi
}

# ============================================================================
# Print Access Information
# ============================================================================

print_access_info() {
    local server_ip
    server_ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "YOUR_SERVER_IP")

    local default_password
    default_password=$(cat "${CONFIG_DIR}/.default_password" 2>/dev/null || echo "UNKNOWN")

    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✅ vpsctl installed successfully!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  ${BOLD}Access URL:${NC}      https://${server_ip}:${DEFAULT_PORT}"
    echo -e "  ${BOLD}Username:${NC}         ${DEFAULT_USER}"
    echo -e "  ${BOLD}Password:${NC}         ${default_password}"
    echo ""
    echo -e "  ${YELLOW}⚠️  IMPORTANT: Change the default password immediately!${NC}"
    echo ""
    echo -e "  ${BOLD}Config File:${NC}     ${CONFIG_DIR}/config.yaml"
    echo -e "  ${BOLD}Data Directory:${NC}  ${DATA_DIR}"
    echo -e "  ${BOLD}Log File:${NC}        ${LOG_DIR}/vpsctl.log"
    echo -e "  ${BOLD}TLS Certs:${NC}       ${CERT_DIR}/"
    echo ""
    echo -e "  ${BOLD}Useful Commands:${NC}"
    echo -e "    Start:       ${CYAN}sudo systemctl start vpsctl${NC}"
    echo -e "    Stop:        ${CYAN}sudo systemctl stop vpsctl${NC}"
    echo -e "    Restart:     ${CYAN}sudo systemctl restart vpsctl${NC}"
    echo -e "    Status:      ${CYAN}sudo systemctl status vpsctl${NC}"
    echo -e "    Logs:        ${CYAN}sudo journalctl -u vpsctl -f${NC}"
    echo -e "    Uninstall:   ${CYAN}sudo bash $(readlink -f "$0") --uninstall${NC}"
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

# ============================================================================
# Uninstall
# ============================================================================

uninstall() {
    echo ""
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  ⚠️  UNINSTALLING vpsctl${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    # Confirm uninstall
    read -rp "Are you sure you want to uninstall vpsctl? (yes/no): " confirm
    if [[ "$confirm" != "yes" ]]; then
        info "Uninstall cancelled"
        exit 0
    fi

    step "Stopping service"
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true

    step "Removing systemd service"
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload

    step "Removing binary"
    rm -f "${INSTALL_DIR}/vpsctl"
    info "Binary removed"

    step "Removing configuration"
    read -rp "Remove configuration directory (${CONFIG_DIR})? (yes/no): " rm_config
    if [[ "$rm_config" == "yes" ]]; then
        rm -rf "$CONFIG_DIR"
        info "Configuration removed"
    else
        info "Configuration preserved at ${CONFIG_DIR}"
    fi

    step "Removing data"
    read -rp "Remove data directory (${DATA_DIR})? This includes ALL data! (yes/no): " rm_data
    if [[ "$rm_data" == "yes" ]]; then
        rm -rf "$DATA_DIR"
        info "Data removed"
    else
        info "Data preserved at ${DATA_DIR}"
    fi

    step "Removing system user"
    if id "vpsctl" &>/dev/null; then
        userdel vpsctl 2>/dev/null || true
        info "User 'vpsctl' removed"
    fi

    echo ""
    info "vpsctl has been uninstalled"
    echo ""
}

# ============================================================================
# Main
# ============================================================================

main() {
    # Parse arguments
    for arg in "$@"; do
        case "$arg" in
            --uninstall|-u)
                check_root
                uninstall
                exit 0
                ;;
            --help|-h)
                echo "Usage: $0 [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  --uninstall, -u    Uninstall vpsctl"
                echo "  --help, -h         Show this help message"
                echo ""
                echo "Environment Variables:"
                echo "  VPSCTL_VERSION     Version to install (default: latest)"
                echo ""
                exit 0
                ;;
        esac
    done

    print_banner
    check_root
    detect_system
    install_dependencies
    check_docker
    create_system_user
    install_binary
    create_directories
    generate_tls_cert
    generate_config
    create_systemd_service
    enable_and_start
    print_access_info
}

main "$@"
