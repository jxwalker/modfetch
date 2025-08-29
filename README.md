# modfetch

Robust CLI/TUI downloader for LLM and Stable Diffusion assets (Hugging Face and CivitAI) with:
- Parallel, chunked downloads with resume and retries
- SHA256 integrity verification
- Automatic artifact classification and placement into user-configured directories
- Batch YAML execution
- Structured logging and rich terminal status

Status: MVP in progress — downloading engine, resolvers, placement, batch, status/verify, metrics, and TUI dashboard are implemented. TUI visuals and config wizard in progress.

Requirements
- Go 1.22+
- Linux (primary), macOS (secondary)

Config
- All configuration is provided via a YAML file. No hard-coded directories are used.
- Pass the config file path with `--config` or via `MODFETCH_CONFIG` env var.
- Generate a starter config via the interactive wizard:
  ```
  modfetch config wizard --out ~/modfetch/config.yml
  ```

Quick start
- Validate config:
  ```
  modfetch config validate --config /path/to/config.yml
  ```
- Download a file (direct URL or resolver URI):
  ```
  modfetch download --config /path/to/config.yml --url 'hf://gpt2/README.md?rev=main'
  modfetch download --config /path/to/config.yml --url 'civitai://model/123456?file=vae'
  ```
- Place a file into your app directories:
  ```
  modfetch place --config /path/to/config.yml --path /path/to/model.safetensors
  ```
- Batch downloads (YAML):
  ```
  modfetch download --config /path/to/config.yml --batch /path/to/jobs.yml --place
  ```
- TUI dashboard:
  ```
  modfetch tui --config /path/to/config.yml
  ```
- Verify files:
  ```
  modfetch verify --config /path/to/config.yml --all
  ```

Project layout
- cmd/modfetch: CLI entrypoint
- internal/config: YAML loader and validation
- internal/downloader: single + chunked engines
- internal/resolver: hf:// and civitai://
- internal/placer: placement engine
- internal/tui: TUI models
- internal/state: SQLite state DB
- internal/metrics: Prometheus textfile metrics
- assets/sample-config: Example configuration files
- docs/: configuration, testing, placement, resolvers guides
- scripts/: smoke test and helpers

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

