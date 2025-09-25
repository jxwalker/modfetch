#!/usr/bin/env bash
set -euo pipefail


INSTALL_DIR=${INSTALL_DIR:-/usr/local/bin}
CONFIG_DIR=${CONFIG_DIR:-"${XDG_CONFIG_HOME:-$HOME/.config}/modfetch"}
DATA_DIR=${DATA_DIR:-"$HOME/modfetch-data"}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { printf "${GREEN}[modfetch-uninstall]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[modfetch-uninstall]${NC} %s\n" "$*"; }
error() { printf "${RED}[modfetch-uninstall]${NC} %s\n" "$*"; }
info() { printf "${BLUE}[modfetch-uninstall]${NC} %s\n" "$*"; }

print_banner() {
    printf "${RED}"
    cat << 'EOF'
    ╔══════════════════════════════════════════════════════════════╗
    ║                                                              ║
    ║                     ModFetch Uninstaller                    ║
    ║                                                              ║
    ║              Remove modfetch from your system               ║
    ║                                                              ║
    ╚══════════════════════════════════════════════════════════════╝
EOF
    printf "${NC}\n"
}

confirm_uninstall() {
    printf "${YELLOW}This will remove modfetch from your system.${NC}\n"
    printf "The following will be removed:\n"
    printf "  - Binary: $INSTALL_DIR/modfetch\n"
    printf "  - Configuration: $CONFIG_DIR\n"
    printf "\nOptionally remove:\n"
    printf "  - Data directory: $DATA_DIR\n"
    printf "  - Shell completions\n"
    printf "\nContinue? [y/N]: "
    read -r confirm
    
    if [[ ! "$confirm" =~ ^[Yy] ]]; then
        info "Uninstallation cancelled"
        exit 0
    fi
}

remove_binary() {
    if [[ -f "$INSTALL_DIR/modfetch" ]]; then
        log "Removing binary from $INSTALL_DIR/modfetch"
        if [[ -w "$INSTALL_DIR" ]]; then
            rm -f "$INSTALL_DIR/modfetch"
        else
            sudo rm -f "$INSTALL_DIR/modfetch"
        fi
        info "Binary removed"
    else
        warn "Binary not found at $INSTALL_DIR/modfetch"
    fi
}

remove_config() {
    if [[ -d "$CONFIG_DIR" ]]; then
        printf "Remove configuration directory $CONFIG_DIR? [Y/n]: "
        read -r remove_conf
        remove_conf=${remove_conf:-y}
        
        if [[ "$remove_conf" =~ ^[Yy] ]]; then
            log "Removing configuration directory"
            rm -rf "$CONFIG_DIR"
            info "Configuration removed"
        else
            info "Configuration preserved"
        fi
    else
        warn "Configuration directory not found"
    fi
}

remove_data() {
    if [[ -d "$DATA_DIR" ]]; then
        printf "${YELLOW}Remove data directory $DATA_DIR? This includes all download history and databases. [y/N]: ${NC}"
        read -r remove_data_dir
        remove_data_dir=${remove_data_dir:-n}
        
        if [[ "$remove_data_dir" =~ ^[Yy] ]]; then
            log "Removing data directory"
            rm -rf "$DATA_DIR"
            info "Data directory removed"
        else
            info "Data directory preserved"
        fi
    else
        warn "Data directory not found"
    fi
}

remove_completions() {
    printf "Remove shell completions? [Y/n]: "
    read -r remove_comp
    remove_comp=${remove_comp:-y}
    
    if [[ "$remove_comp" =~ ^[Yy] ]]; then
        log "Removing shell completions"
        
        rm -f "$HOME/.local/share/bash-completion/completions/modfetch"
        
        rm -f "$HOME/.local/share/zsh/site-functions/_modfetch"
        
        rm -f "$HOME/.config/fish/completions/modfetch.fish"
        
        info "Shell completions removed"
    else
        info "Shell completions preserved"
    fi
}

cleanup_path() {
    printf "Remove $INSTALL_DIR from shell PATH? [y/N]: "
    read -r cleanup_path_var
    cleanup_path_var=${cleanup_path_var:-n}
    
    if [[ "$cleanup_path_var" =~ ^[Yy] ]]; then
        warn "Please manually remove the following line from your shell configuration:"
        warn "export PATH=\"$INSTALL_DIR:\$PATH\""
        info "Common locations: ~/.bashrc, ~/.zshrc, ~/.config/fish/config.fish"
    fi
}

main() {
    print_banner
    
    log "Starting modfetch uninstallation..."
    
    confirm_uninstall
    
    remove_binary
    remove_config
    remove_data
    remove_completions
    
    if [[ "$INSTALL_DIR" != "/usr/local/bin" ]]; then
        cleanup_path
    fi
    
    printf "\n${GREEN}✓ Uninstallation completed${NC}\n"
    info "Thank you for using modfetch!"
}

main "$@"
