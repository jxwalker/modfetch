# modfetch

Robust CLI/TUI downloader for LLM and Stable Diffusion assets (Hugging Face and CivitAI) with:
- Parallel, chunked downloads with resume and retries
- SHA256 integrity verification
- Automatic artifact classification and placement into user-configured directories
- Batch YAML execution
- Structured logging and rich terminal status

Status: Milestone M0 scaffold

Requirements
- Go 1.22+
- Linux (primary), macOS (secondary)

Config
- All configuration is provided via a YAML file. No hard-coded directories are used.
- Pass the config file path with `--config` or via `MODFETCH_CONFIG` env var.

Example usage (skeleton)
- Validate config:
  ```
  modfetch config validate --config /path/to/config.yml
  ```
- Print config (round-trip):
  ```
  modfetch config print --config /path/to/config.yml
  ```

Project layout
- cmd/modfetch: CLI entrypoint
- internal/config: YAML loader and validation
- assets/sample-config: Example configuration files

Installation (from source)
- Build: `make build`
- Tests: `make test`
- Cross-compile and package: `make release-dist`

GitHub Releases (CI)
- Tag a version like `v0.1.0` to trigger the release workflow. Binaries for Linux and macOS (amd64/arm64) will be attached to the release with checksums.

Homebrew tap (template)
- See `packaging/homebrew/modfetch.rb` for a formula template. After creating a release, update version and checksums and publish to your tap repo.

Next milestones
- M0: CLI skeleton + config loader
- M1: Single-stream downloader + SHA256 + SQLite state
- M2: Parallel chunked downloads + retries/backoff
- M3: Hugging Face resolver
- M4: CivitAI resolver
- M5: Classifier + placement engine
- M6: Batch YAML + verify + status
- M7: TUI dashboard
- M8: Metrics & performance
- M9: Packaging & release (this)

