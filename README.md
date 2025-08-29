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

Next milestones
- M0: CLI skeleton + config loader (this commit)
- M1: Single-stream downloader + SHA256 + SQLite state
- M2: Parallel chunked downloads + retries/backoff
- M3: Hugging Face resolver
- M4: CivitAI resolver
- M5: Classifier + placement engine
- M6: Batch YAML + verify + status
- M7: TUI dashboard
- M8: Metrics & performance
- M9: Packaging & release

