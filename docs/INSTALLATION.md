# Installation Guide

This guide covers all installation methods for modfetch across Linux and macOS platforms.

## Quick Install (Recommended)

### One-liner Installation
```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash
```

This will:
- Download the latest release binary for your platform
- Install to `/usr/local/bin` (or `~/bin` if not writable)
- Run the interactive configuration wizard
- Set up shell completions
- Perform a smoke test

### Custom Installation Directory
```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash -s -- --install-dir ~/bin
```

### Skip Configuration Wizard
```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash -s -- --skip-config-wizard
```

## Manual Installation

### From GitHub Releases
1. Visit https://github.com/jxwalker/modfetch/releases
2. Download the appropriate binary for your platform:
   - Linux: `modfetch_linux_amd64` or `modfetch_linux_arm64`
   - macOS: `modfetch_darwin_universal` (recommended) or architecture-specific
3. Make executable: `chmod +x modfetch_*`
4. Move to PATH: `sudo mv modfetch_* /usr/local/bin/modfetch`
5. Run config wizard: `modfetch config wizard --out ~/.config/modfetch/config.yml`

### From Source
```bash
git clone https://github.com/jxwalker/modfetch.git
cd modfetch
make build
sudo cp bin/modfetch /usr/local/bin/
```

## Platform-Specific Instructions

### macOS
- **Homebrew** (coming soon): `brew install jxwalker/tap/modfetch`
- **Manual**: Use `modfetch_darwin_universal` for best compatibility

### Linux
- **Ubuntu/Debian**: One-liner installer handles all dependencies
- **CentOS/RHEL**: Ensure `curl` and `tar` are installed first
- **Arch Linux**: AUR package coming soon

## Configuration

After installation, modfetch needs a configuration file. The installer will run the interactive configuration wizard automatically, or you can run it manually:

```bash
modfetch config wizard --out ~/.config/modfetch/config.yml
```

### Manual Configuration
Create a minimal config at `~/.config/modfetch/config.yml`:

```yaml
version: 1
general:
  data_root: "~/modfetch-data"
  download_root: "~/Downloads/modfetch"
  placement_mode: "symlink"
network:
  timeout_seconds: 60
concurrency:
  per_file_chunks: 4
  chunk_size_mb: 8
sources:
  huggingface: { enabled: true, token_env: "HF_TOKEN" }
  civitai:     { enabled: true, token_env: "CIVITAI_TOKEN" }
```

### Environment Variables
For private/gated content, set these environment variables:
- `HF_TOKEN` - Hugging Face access token
- `CIVITAI_TOKEN` - CivitAI API token

## Verification

Test your installation:

```bash
# Check version
modfetch version

# Validate configuration
modfetch config validate --config ~/.config/modfetch/config.yml

# Test download (public file)
modfetch download --config ~/.config/modfetch/config.yml --url 'https://proof.ovh.net/files/1Mb.dat'

# Test TUI
modfetch tui --config ~/.config/modfetch/config.yml
```

## Uninstallation

```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/uninstall.sh | bash
```

This will:
- Remove the modfetch binary
- Optionally remove configuration and data directories
- Clean up shell completions
- Provide guidance for PATH cleanup

## Troubleshooting

### Permission Issues
- **Permission denied during install**: Try installing to `~/bin` instead:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash -s -- --install-dir ~/bin
  ```
- **Command not found**: Ensure installation directory is in your PATH

### Configuration Issues
- **Config wizard fails**: Run manually:
  ```bash
  modfetch config wizard --out ~/.config/modfetch/config.yml
  ```
- **Invalid config**: Validate with:
  ```bash
  modfetch config validate --config ~/.config/modfetch/config.yml
  ```

### Network Issues
- **Download failures**: Check network connectivity and proxy settings
- **Auth failures**: Verify `HF_TOKEN` and `CIVITAI_TOKEN` environment variables

### Platform-Specific Issues

#### macOS
- **"modfetch cannot be opened"**: Run `xattr -d com.apple.quarantine /usr/local/bin/modfetch`
- **Architecture mismatch**: Use `modfetch_darwin_universal` for compatibility

#### Linux
- **Missing dependencies**: Install `curl` and `tar`:
  ```bash
  # Ubuntu/Debian
  sudo apt-get update && sudo apt-get install -y curl tar
  
  # CentOS/RHEL
  sudo yum install -y curl tar
  ```

## Developer Installation

For development work, use the enhanced setup script:

```bash
git clone https://github.com/jxwalker/modfetch.git
cd modfetch
./scripts/setup-dev.sh
```

This will:
- Install Go 1.22+ if needed
- Set up development tools (golangci-lint, goimports)
- Configure git hooks for pre-commit checks
- Set up VS Code configuration
- Run initial build and tests

## Getting Help

- **Documentation**: See `docs/` directory for detailed guides
- **Issues**: Report bugs at https://github.com/jxwalker/modfetch/issues
- **Discussions**: Ask questions at https://github.com/jxwalker/modfetch/discussions
