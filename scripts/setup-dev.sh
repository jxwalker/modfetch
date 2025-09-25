#!/usr/bin/env bash
set -euo pipefail


GO_VERSION=${GO_VERSION:-1.22.10}
GOLANGCI_LINT_VERSION=${GOLANGCI_LINT_VERSION:-v1.55.2}

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { printf "${GREEN}[dev-setup]${NC} %s\n" "$*" >&2; }
warn() { printf "${YELLOW}[dev-setup]${NC} %s\n" "$*" >&2; }
info() { printf "${BLUE}[dev-setup]${NC} %s\n" "$*" >&2; }

have_cmd() { command -v "$1" >/dev/null 2>&1; }

print_banner() {
    printf "${BLUE}" >&2
    cat << 'EOF' >&2
    ╔══════════════════════════════════════════════════════════════╗
    ║                                                              ║
    ║                ModFetch Development Setup                    ║
    ║                                                              ║
    ║         Setting up your development environment              ║
    ║                                                              ║
    ╚══════════════════════════════════════════════════════════════╝
EOF
    printf "${NC}\n" >&2
}

check_go() {
    if have_cmd go; then
        local ver; ver=$(go version | awk '{print $3}' | sed 's/go//')
        local major minor
        major=$(echo "$ver" | cut -d. -f1)
        minor=$(echo "$ver" | cut -d. -f2)
        if [[ "${major:-0}" -gt 1 ]] || { [[ "${major:-0}" -eq 1 ]] && [[ "${minor:-0}" -ge 22 ]]; }; then
            log "Go $(go version | awk '{print $3}') is installed and compatible"
            return 0
        fi
    fi
    
    warn "Go 1.22+ is required but not found"
    info "Please install Go from https://golang.org/dl/"
    return 1
}

setup_git_hooks() {
    if [[ ! -d .git ]]; then
        warn "Not in a git repository, skipping git hooks setup"
        return
    fi
    
    log "Setting up git hooks..."
    
    cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
set -e

echo "Running pre-commit checks..."

if ! make fmt-check; then
    echo "❌ Code formatting issues found. Run 'make fmt' to fix."
    exit 1
fi

if ! make lint; then
    echo "❌ Linting issues found. Please fix before committing."
    exit 1
fi

if ! make vet; then
    echo "❌ Go vet issues found. Please fix before committing."
    exit 1
fi

if ! make test; then
    echo "❌ Tests failed. Please fix before committing."
    exit 1
fi

echo "✅ All pre-commit checks passed"
EOF
    
    chmod +x .git/hooks/pre-commit
    info "Pre-commit hook installed"
}

install_tools() {
    log "Installing development tools..."
    
    if ! have_cmd golangci-lint; then
        log "Installing golangci-lint $GOLANGCI_LINT_VERSION"
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin $GOLANGCI_LINT_VERSION
    else
        info "golangci-lint already installed"
    fi
    
    go install golang.org/x/tools/cmd/goimports@latest
    go install github.com/goreleaser/goreleaser@latest
    
    info "Development tools installed"
}

setup_vscode() {
    if [[ ! -d .vscode ]]; then
        mkdir -p .vscode
    fi
    
    cat > .vscode/settings.json << 'EOF'
{
    "go.useLanguageServer": true,
    "go.lintTool": "golangci-lint",
    "go.lintFlags": ["--fast"],
    "go.formatTool": "goimports",
    "go.testFlags": ["-v"],
    "go.testTimeout": "30s",
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
        "source.organizeImports": true
    },
    "files.exclude": {
        "**/bin": true,
        "**/dist": true,
        "**/.DS_Store": true
    }
}
EOF
    
    cat > .vscode/extensions.json << 'EOF'
{
    "recommendations": [
        "golang.go",
        "ms-vscode.vscode-json",
        "redhat.vscode-yaml",
        "ms-vscode.makefile-tools"
    ]
}
EOF
    
    info "VS Code configuration created"
}

run_initial_build() {
    log "Running initial build and tests..."
    
    go mod download
    go mod tidy
    
    make build
    
    make test
    
    make lint
    
    info "Initial build and tests completed successfully"
}

print_next_steps() {
    printf "\n${GREEN}🎉 Development environment setup completed!${NC}\n\n" >&2
    
    printf "${BLUE}Next Steps:${NC}\n" >&2
    printf "1. ${GREEN}Build and test:${NC}\n" >&2
    printf "   make build && make test\n\n" >&2
    
    printf "2. ${GREEN}Run the application:${NC}\n" >&2
    printf "   ./bin/modfetch --help\n\n" >&2
    
    printf "3. ${GREEN}Development workflow:${NC}\n" >&2
    printf "   make fmt      # Format code\n" >&2
    printf "   make lint     # Run linter\n" >&2
    printf "   make test     # Run tests\n" >&2
    printf "   make build    # Build binary\n\n" >&2
    
    printf "4. ${GREEN}Release workflow:${NC}\n" >&2
    printf "   make release-dist  # Build cross-platform binaries\n\n" >&2
    
    printf "${BLUE}Git hooks are installed and will run checks before each commit.${NC}\n" >&2
    printf "${BLUE}VS Code configuration is set up for optimal Go development.${NC}\n\n" >&2
    
    printf "Happy coding! 🚀\n" >&2
}

main() {
    print_banner
    
    if ! check_go; then
        exit 1
    fi
    
    install_tools
    setup_git_hooks
    setup_vscode
    run_initial_build
    
    print_next_steps
}

main "$@"
