#!/usr/bin/env bash
set -euo pipefail

#

VERSION=${MODFETCH_VERSION:-latest}
INSTALL_DIR=${INSTALL_DIR:-/usr/local/bin}
CONFIG_DIR=${CONFIG_DIR:-"${XDG_CONFIG_HOME:-$HOME/.config}/modfetch"}
DATA_DIR=${DATA_DIR:-"$HOME/modfetch-data"}
DOWNLOAD_DIR=${DOWNLOAD_DIR:-"$HOME/Downloads/modfetch"}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

log() { printf "${GREEN}[modfetch-install]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[modfetch-install]${NC} %s\n" "$*"; }
error() { printf "${RED}[modfetch-install]${NC} %s\n" "$*"; }
info() { printf "${BLUE}[modfetch-install]${NC} %s\n" "$*"; }
success() { printf "${GREEN}✓${NC} %s\n" "$*"; }

have_cmd() { command -v "$1" >/dev/null 2>&1; }
is_root() { [[ $EUID -eq 0 ]]; }

detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        echo "linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        echo "darwin"
    else
        error "Unsupported OS: $OSTYPE"
        exit 1
    fi
}

detect_arch() {
    case "$(uname -m)" in
        x86_64) echo "amd64";;
        aarch64|arm64) echo "arm64";;
        *) error "Unsupported architecture: $(uname -m)"; exit 1;;
    esac
}

print_banner() {
    printf "${PURPLE}"
    cat << 'EOF'
    ╔══════════════════════════════════════════════════════════════╗
    ║                                                              ║
    ║                        ModFetch Installer                   ║
    ║                                                              ║
    ║    Fetch, verify, and place LLM and Stable Diffusion       ║
    ║    models with ease. Supports HuggingFace, CivitAI,        ║
    ║    and direct HTTP downloads.                               ║
    ║                                                              ║
    ╚══════════════════════════════════════════════════════════════╝
EOF
    printf "${NC}\n"
}

check_prerequisites() {
    log "Checking prerequisites..."
    
    local missing=()
    
    if ! have_cmd curl && ! have_cmd wget; then
        missing+=("curl or wget")
    fi
    
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing required tools: ${missing[*]}"
        info "Please install the missing tools and run the installer again."
        
        if [[ "$(detect_os)" == "linux" ]]; then
            info "On Ubuntu/Debian: sudo apt-get update && sudo apt-get install -y curl"
            info "On CentOS/RHEL: sudo yum install -y curl"
        elif [[ "$(detect_os)" == "darwin" ]]; then
            info "On macOS: curl is usually pre-installed"
        fi
        exit 1
    fi
    
    success "Prerequisites check passed"
}

get_latest_version() {
    if [[ "$VERSION" == "latest" ]]; then
        log "Fetching latest version from GitHub..."
        if have_cmd curl; then
            VERSION=$(curl -fsSL https://api.github.com/repos/jxwalker/modfetch/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        elif have_cmd wget; then
            VERSION=$(wget -qO- https://api.github.com/repos/jxwalker/modfetch/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        fi
        
        if [[ -z "$VERSION" ]]; then
            warn "Could not fetch latest version, using v0.5.0 as fallback"
            VERSION="v0.5.0"
        fi
    fi
    
    info "Installing modfetch $VERSION"
}

download_binary() {
    local os arch binary_name download_url temp_file
    os=$(detect_os)
    arch=$(detect_arch)
    binary_name="modfetch_${os}_${arch}"
    download_url="https://github.com/jxwalker/modfetch/releases/download/${VERSION}/${binary_name}"
    temp_file="/tmp/modfetch_${VERSION}_${os}_${arch}"
    
    echo "DEBUG: temp_file path will be: $temp_file" >&2
    
    log "Downloading modfetch binary from $download_url" >&2
    
    if have_cmd curl; then
        curl -fsSL "$download_url" -o "$temp_file"
    elif have_cmd wget; then
        wget -q "$download_url" -O "$temp_file"
    fi
    
    if [[ ! -f "$temp_file" ]]; then
        error "Failed to download modfetch binary" >&2
        exit 1
    fi
    
    success "Downloaded modfetch binary" >&2
    
    echo "DEBUG: File exists check: $(ls -la "$temp_file" 2>/dev/null || echo 'FILE NOT FOUND')" >&2
    
    echo "$temp_file"
}

install_binary() {
    local temp_file="$1"
    
    log "Installing modfetch to $INSTALL_DIR"
    
    if [[ ! -d "$INSTALL_DIR" ]]; then
        if [[ -w "$(dirname "$INSTALL_DIR")" ]]; then
            mkdir -p "$INSTALL_DIR"
        else
            info "Creating $INSTALL_DIR requires sudo privileges"
            sudo mkdir -p "$INSTALL_DIR"
        fi
    fi
    
    if [[ -w "$INSTALL_DIR" ]]; then
        cp "$temp_file" "$INSTALL_DIR/modfetch"
        chmod +x "$INSTALL_DIR/modfetch"
    else
        info "Installing to $INSTALL_DIR requires sudo privileges"
        sudo cp "$temp_file" "$INSTALL_DIR/modfetch"
        sudo chmod +x "$INSTALL_DIR/modfetch"
    fi
    
    rm -f "$temp_file"
    
    success "Installed modfetch to $INSTALL_DIR/modfetch"
}

create_directories() {
    log "Creating configuration and data directories..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$DOWNLOAD_DIR"
    
    success "Created directories:"
    info "  Config: $CONFIG_DIR"
    info "  Data: $DATA_DIR"
    info "  Downloads: $DOWNLOAD_DIR"
}

interactive_config() {
    local config_file="$CONFIG_DIR/config.yml"
    
    if [[ "$SKIP_CONFIG_WIZARD" == "true" ]]; then
        info "Skipping config wizard (--skip-config-wizard specified)"
        create_minimal_config
        return
    fi
    
    if [[ -f "$config_file" ]]; then
        printf "${YELLOW}Configuration file already exists at $config_file${NC}\n"
        printf "Do you want to:\n"
        printf "  1) Keep existing configuration\n"
        printf "  2) Create new configuration (backup existing)\n"
        printf "  3) Run configuration wizard\n"
        printf "Choice [1]: "
        read -r choice
        choice=${choice:-1}
        
        case "$choice" in
            1) 
                info "Keeping existing configuration"
                return
                ;;
            2)
                warn "Backing up existing configuration to $config_file.backup"
                cp "$config_file" "$config_file.backup"
                ;;
            3)
                if [[ -x "$INSTALL_DIR/modfetch" ]]; then
                    log "Running configuration wizard..."
                    if ! "$INSTALL_DIR/modfetch" config wizard --out "$config_file"; then
                        warn "Config wizard failed, creating basic configuration instead"
                    else
                        info "Configuration saved to $config_file"
                        info "You can reconfigure anytime with: modfetch config wizard --out $config_file"
                        return
                    fi
                else
                    warn "modfetch not found, creating basic configuration"
                fi
                ;;
        esac
    else
        printf "Run interactive configuration wizard? [Y/n]: "
        read -r run_wizard
        run_wizard=${run_wizard:-y}
        
        if [[ "$run_wizard" =~ ^[Yy] ]]; then
            if [[ -x "$INSTALL_DIR/modfetch" ]]; then
                log "Running configuration wizard..."
                if ! "$INSTALL_DIR/modfetch" config wizard --out "$config_file"; then
                    warn "Config wizard failed, creating basic configuration instead"
                else
                    info "Configuration saved to $config_file"
                    info "You can reconfigure anytime with: modfetch config wizard --out $config_file"
                    return
                fi
            else
                warn "modfetch not found, creating basic configuration"
            fi
        fi
    fi
    
    create_minimal_config
}

create_minimal_config() {
    local config_file="$CONFIG_DIR/config.yml"
    log "Creating minimal configuration file..."
    
    local user_download_dir="$DOWNLOAD_DIR"
    local user_data_dir="$DATA_DIR"
    local enable_hf="y"
    local enable_civitai="y"
    local require_sha256="n"
    
    if [[ "$SKIP_CONFIG_WIZARD" != "true" ]]; then
        printf "${CYAN}Configuration Setup${NC}\n"
        printf "Press Enter to use default values shown in [brackets]\n\n"
        
        printf "Download directory [$DOWNLOAD_DIR]: "
        read -r user_download_dir
        user_download_dir=${user_download_dir:-$DOWNLOAD_DIR}
        
        printf "Data directory [$DATA_DIR]: "
        read -r user_data_dir
        user_data_dir=${user_data_dir:-$DATA_DIR}
        
        printf "Enable HuggingFace integration? [Y/n]: "
        read -r enable_hf
        enable_hf=${enable_hf:-y}
        
        printf "Enable CivitAI integration? [Y/n]: "
        read -r enable_civitai
        enable_civitai=${enable_civitai:-y}
        
        printf "Require SHA256 verification? [y/N]: "
        read -r require_sha256
        require_sha256=${require_sha256:-n}
    fi
    
    cat > "$config_file" << EOF
version: 1
general:
  data_root: "$user_data_dir"
  download_root: "$user_download_dir"
  placement_mode: "symlink"
  quarantine: true
  allow_overwrite: false
network:
  timeout_seconds: 60
  max_redirects: 10
  tls_verify: true
  user_agent: "modfetch/$VERSION"
concurrency:
  global_files: 4
  per_file_chunks: 4
  chunk_size_mb: 8
  max_retries: 8
  backoff:
    min_ms: 200
    max_ms: 30000
    jitter: true
sources:
  huggingface:
    enabled: $(if [[ "$enable_hf" =~ ^[Yy] ]]; then echo "true"; else echo "false"; fi)
    token_env: "HF_TOKEN"
  civitai:
    enabled: $(if [[ "$enable_civitai" =~ ^[Yy] ]]; then echo "true"; else echo "false"; fi)
    token_env: "CIVITAI_TOKEN"
placement:
  apps: {}
  mapping: []
logging:
  level: "info"
  format: "human"
  file:
    enabled: false
    path: ""
    max_megabytes: 50
    max_backups: 3
    max_age_days: 14
metrics:
  prometheus_textfile:
    enabled: false
    path: ""
validation:
  require_sha256: $(if [[ "$require_sha256" =~ ^[Yy] ]]; then echo "true"; else echo "false"; fi)
  accept_md5_sha1_if_provided: false
EOF
    
    success "Created configuration file at $config_file"
}

setup_shell_integration() {
    log "Setting up shell integration..."
    
    local shell_rc
    case "$SHELL" in
        */bash) shell_rc="$HOME/.bashrc";;
        */zsh) shell_rc="$HOME/.zshrc";;
        */fish) shell_rc="$HOME/.config/fish/config.fish";;
        *) 
            warn "Unknown shell: $SHELL, skipping shell integration"
            return
            ;;
    esac
    
    if [[ "$INSTALL_DIR" != "/usr/local/bin" ]] && [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        printf "Add $INSTALL_DIR to PATH in $shell_rc? [Y/n]: "
        read -r add_path
        add_path=${add_path:-y}
        
        if [[ "$add_path" =~ ^[Yy] ]]; then
            echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$shell_rc"
            success "Added $INSTALL_DIR to PATH in $shell_rc"
            info "Run 'source $shell_rc' or restart your shell to apply changes"
        fi
    fi
    
    if have_cmd modfetch || [[ -x "$INSTALL_DIR/modfetch" ]]; then
        local modfetch_bin
        if have_cmd modfetch; then
            modfetch_bin="modfetch"
        else
            modfetch_bin="$INSTALL_DIR/modfetch"
        fi
        
        printf "Setup shell completions? [Y/n]: "
        read -r setup_completions
        setup_completions=${setup_completions:-y}
        
        if [[ "$setup_completions" =~ ^[Yy] ]]; then
            case "$SHELL" in
                */bash)
                    mkdir -p "$HOME/.local/share/bash-completion/completions"
                    "$modfetch_bin" completion bash > "$HOME/.local/share/bash-completion/completions/modfetch"
                    success "Installed bash completions"
                    ;;
                */zsh)
                    mkdir -p "$HOME/.local/share/zsh/site-functions"
                    "$modfetch_bin" completion zsh > "$HOME/.local/share/zsh/site-functions/_modfetch"
                    success "Installed zsh completions"
                    ;;
                */fish)
                    mkdir -p "$HOME/.config/fish/completions"
                    "$modfetch_bin" completion fish > "$HOME/.config/fish/completions/modfetch.fish"
                    success "Installed fish completions"
                    ;;
            esac
        fi
    fi
}

run_smoke_test() {
    local modfetch_bin config_file
    
    if have_cmd modfetch; then
        modfetch_bin="modfetch"
    elif [[ -x "$INSTALL_DIR/modfetch" ]]; then
        modfetch_bin="$INSTALL_DIR/modfetch"
    else
        error "modfetch binary not found"
        return 1
    fi
    
    config_file="$CONFIG_DIR/config.yml"
    
    log "Running smoke tests..."
    
    if ! "$modfetch_bin" version; then
        error "Version check failed"
        return 1
    fi
    success "Version check passed"
    
    if ! "$modfetch_bin" config validate --config "$config_file"; then
        error "Configuration validation failed"
        return 1
    fi
    success "Configuration validation passed"
    
    printf "${CYAN}Run a test download? This will download a small 1MB test file. [Y/n]: ${NC}"
    read -r run_test_download
    run_test_download=${run_test_download:-y}
    
    if [[ "$run_test_download" =~ ^[Yy] ]]; then
        log "Running test download (1MB file)..."
        if "$modfetch_bin" download --config "$config_file" --url 'https://proof.ovh.net/files/1Mb.dat'; then
            success "Test download completed successfully"
        else
            warn "Test download failed, but installation appears successful"
        fi
    fi
}

print_next_steps() {
    local config_file="$CONFIG_DIR/config.yml"
    
    printf "\n${GREEN}🎉 Installation completed successfully!${NC}\n\n"
    
    printf "${WHITE}Next Steps:${NC}\n"
    printf "1. ${CYAN}Verify installation:${NC}\n"
    printf "   modfetch version\n\n"
    
    printf "2. ${CYAN}Configure tokens (if needed):${NC}\n"
    printf "   export HF_TOKEN='your_huggingface_token'     # For HuggingFace\n"
    printf "   export CIVITAI_TOKEN='your_civitai_token'    # For CivitAI\n\n"
    
    printf "3. ${CYAN}Try some downloads:${NC}\n"
    printf "   # Direct HTTP download\n"
    printf "   modfetch download --url 'https://proof.ovh.net/files/1Mb.dat'\n\n"
    printf "   # HuggingFace model\n"
    printf "   modfetch download --url 'hf://gpt2/README.md?rev=main'\n\n"
    printf "   # CivitAI model (requires token)\n"
    printf "   modfetch download --url 'civitai://model/123456'\n\n"
    
    printf "4. ${CYAN}Launch TUI dashboard:${NC}\n"
    printf "   modfetch tui\n\n"
    
    printf "5. ${CYAN}View status and verify downloads:${NC}\n"
    printf "   modfetch status\n"
    printf "   modfetch verify --all\n\n"
    
    printf "${WHITE}Configuration:${NC}\n"
    printf "  Config file: $config_file\n"
    printf "  Data directory: $DATA_DIR\n"
    printf "  Download directory: $DOWNLOAD_DIR\n\n"
    
    printf "${WHITE}Documentation:${NC}\n"
    printf "  GitHub: https://github.com/jxwalker/modfetch\n"
    printf "  User Guide: https://github.com/jxwalker/modfetch/blob/main/docs/USER_GUIDE.md\n\n"
    
    printf "${YELLOW}Need help? Open an issue at: https://github.com/jxwalker/modfetch/issues${NC}\n"
}

cleanup_on_error() {
    error "Installation failed. Cleaning up..."
    rm -f "/tmp/modfetch_"*
    exit 1
}

main() {
    trap cleanup_on_error ERR
    
    print_banner
    
    log "Starting modfetch installation..."
    info "OS: $(detect_os)"
    info "Architecture: $(detect_arch)"
    info "Install directory: $INSTALL_DIR"
    info "Config directory: $CONFIG_DIR"
    
    check_prerequisites
    get_latest_version
    
    local temp_file
    echo "DEBUG: About to call download_binary" >&2
    temp_file=$(download_binary)
    echo "DEBUG: download_binary returned: '$temp_file'" >&2
    echo "DEBUG: Length of temp_file: ${#temp_file}" >&2
    install_binary "$temp_file"
    
    create_directories
    interactive_config
    setup_shell_integration
    
    if [[ "$SKIP_CONFIG_WIZARD" != "true" ]]; then
        run_smoke_test
    else
        info "Skipping smoke test (--skip-config-wizard specified)"
    fi
    
    print_next_steps
    
    success "Installation completed successfully!"
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --config-dir)
            CONFIG_DIR="$2"
            shift 2
            ;;
        --data-dir)
            DATA_DIR="$2"
            shift 2
            ;;
        --download-dir)
            DOWNLOAD_DIR="$2"
            shift 2
            ;;
        --help)
            cat << EOF
ModFetch Installation Script

Usage: $0 [OPTIONS]

Options:
  --version VERSION       Install specific version (default: latest)
  --install-dir DIR       Binary installation directory (default: /usr/local/bin)
  --config-dir DIR        Configuration directory (default: ~/.config/modfetch)
  --data-dir DIR          Data directory (default: ~/modfetch-data)
  --download-dir DIR      Download directory (default: ~/Downloads/modfetch)
  --skip-config-wizard    Skip interactive configuration wizard
  --help                  Show this help message

Environment Variables:
  MODFETCH_VERSION        Version to install (default: latest)
  INSTALL_DIR             Binary installation directory
  CONFIG_DIR              Configuration directory
  DATA_DIR                Data directory
  DOWNLOAD_DIR            Download directory

Examples:
  $0
  
  $0 --version v0.4.0
  
  $0 --install-dir ~/bin
  
  curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash
  
  $0 --skip-config-wizard
EOF
            exit 0
            ;;
        --skip-config-wizard)
            SKIP_CONFIG_WIZARD="true"
            shift
            ;;
        *)
            error "Unknown option: $1"
            exit 1
            ;;
    esac
done

main "$@"
