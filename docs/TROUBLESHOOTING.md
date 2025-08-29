# Troubleshooting

Common errors and remedies

- No space left on device
  - Message: "write failed: no space left on device"
  - Action: free disk space on the download filesystem; modfetch writes to general.download_root and uses a .part file before renaming.

- Server ignored Range; restarting from beginning
  - Message: "server ignored Range; restarting from beginning"
  - Action: the server does not support Range for resuming; modfetch restarts the single-stream download from zero.

- HEAD status not OK; falling back to single
  - Message: "chunked: falling back to single: ..."
  - Action: Some hosts block HEAD or do not advertise Accept-Ranges. modfetch falls back to single-stream automatically.

- Checksum mismatch
  - Single: "sha256 mismatch: expected=... actual=..."
  - Chunked: "sha256 mismatch after repair: expected=... got=..."
  - Action: verify the expected SHA256; if provided by upstream, re-run. For chunked downloads, modfetch re-hashes chunks and re-fetches corrupted ranges automatically before failing.

- Permission denied creating directories
  - Message: errors when creating directories under general.download_root or placement targets
  - Action: ensure the user has write permissions to the configured paths. For placement, you may need to adjust your apps base directories or run with proper permissions.

- Missing tokens
  - Message: 401/403 from huggingface.co or civitai.com
  - Action: export HF_TOKEN or CIVITAI_TOKEN in your shell if accessing gated resources. Do not store secrets in YAML.

Diagnostic tips
- Increase verbosity: use --log-level debug
- Use --json to get structured logs for programmatic analysis
- Use --summary-json to emit a single JSON summary per completed download
- TUI: press h or ? for help; s to sort by speed, e to sort by ETA

