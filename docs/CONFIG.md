# Configuration guide

All configuration is provided via a single YAML file. No secrets are stored in the file; tokens are read from environment variables.

Quick start: interactive wizard
- Generate a starter config interactively:
  ```
  modfetch config wizard --out ~/modfetch/config.yml
  ```
- Or print to stdout:
  ```
  modfetch config wizard
  ```

## Top-level schema (version: 1)
- `general`
  - `data_root`: State DB, logs, metrics root
  - `download_root`: Where partial and finished downloads are staged before placement
  - `placement_mode`: `symlink` | `hardlink` | `copy`
  - `quarantine`: true|false (quarantine until checksum verified)
  - `allow_overwrite`: true|false
  - `stage_partials`: true|false (default true). When true, .part files are written under `download_root/.parts` (or `partials_root` if set) and moved atomically on completion.
  - `partials_root`: string (optional). Directory to store .part files instead of `download_root/.parts` (useful for a faster or larger filesystem).
  - `always_no_resume`: true|false (default false). When true, every download starts fresh (ignores any .part and clears chunk state) unless overridden by CLI.
  - `auto_recover_on_start`: true|false (default false). When true, the TUI auto-resumes downloads found in state with status `running` or `hold` on startup.
- `network`
  - `timeout_seconds`, `max_redirects`, `tls_verify`, `user_agent`
  - `retry_on_rate_limit`: true|false. When true, honor HTTP 429 Retry-After to determine wait between retries.
  - `rate_limit_max_delay_seconds`: integer (>=0). Caps the wait derived from Retry-After (default cap 600s if unset).
  - `disable_auth_preflight`: true|false. When true, skip the early HEAD/0–0 preflight in CLI and TUI v2 (default is enabled; disable to avoid HEAD on some hosts).
- `concurrency`
  - `global_files`, `per_file_chunks`, `per_host_requests`, `chunk_size_mb`, `max_retries`, `backoff`
- `sources`
  - `huggingface`: `enabled`, `token_env`
  - `civitai`: `enabled`, `token_env`
- `resolver`
  - `cache_ttl_hours`: Hours to keep resolver results before re-querying (default 24)
- `placement`
  - `apps`: map of app names → base + relative paths
  - `mapping`: artifact type → list of targets (app/path_key)
- `classifier`
  - `rules`: list of regex → type entries evaluated before built-in detection
- `logging`
  - `level`, `format`, `file`: path and rotation
- `metrics`
  - `prometheus_textfile`: `enabled`, `path`
- `validation`
  - `require_sha256`, `accept_md5_sha1_if_provided`
  - `safetensors_deep_verify_after_download`: when true, perform deep coverage/length verification of .safetensors files immediately after download; fail the command if invalid
- `ui`
  - `refresh_hz`, `column_mode`, `compact`, `theme`

See `assets/sample-config/config.example.yml` for a full example.

## Tokens / environment variables
- Set in environment, not in YAML:
  - `HF_TOKEN`: Hugging Face token (Bearer), used when sources.huggingface.enabled is true
- `CIVITAI_TOKEN`: CivitAI token (Bearer), used when sources.civitai.enabled is true
- Do not print secrets back to the terminal; export them in your shell profile or a secure env file

## Resolver cache
- Resolved URIs are cached in `resolver-cache.json` under `data_root`.
- Entries expire after `resolver.cache_ttl_hours` (default 24); 0 disables caching.
- Cache entries are refreshed on 404 responses or when expired.

## Placement mapping
- Map artifact types to target apps/paths. Common types:
  - `sd.checkpoint`, `sd.lora`, `sd.vae`, `sd.controlnet`, `sd.embedding`, `llm.gguf`, `llm.safetensors`, `generic`.
- Example:
```
placement:
  apps:
    comfyui:
      base: "/home/user/ComfyUI"
      paths:
        checkpoints: "models/checkpoints"
        lora: "models/loras"
    a1111:
      base: "/home/user/stable-diffusion-webui"
      paths:
        checkpoints: "models/Stable-diffusion"
        lora: "models/Lora"
  mapping:
    - match: sd.checkpoint
      targets:
        - app: comfyui
          path_key: checkpoints
        - app: a1111
          path_key: checkpoints
    - match: sd.lora
      targets:
        - app: comfyui
          path_key: lora
        - app: a1111
          path_key: lora
```

## UI options

Configure visual aspects of the TUI under the `ui` section:

```yaml
ui:
  refresh_hz: 1            # refresh rate (0-10)
  column_mode: dest        # dest | url | host
  compact: false           # compact table view
  theme:                  # optional color overrides (8-bit codes)
    border: "63"
    title: "81"
    tab_active: "219"
    tab_inactive: "240"
    head: "213"
    ok: "42"
    bad: "196"
```

Press `T` in the TUI to cycle built-in themes; the selection is saved to `ui_state_v2.json`.


## Classifier overrides

`classifier.rules` lets you override artifact type detection. Each rule provides a
regular expression and the type to return when the pattern matches the file's
basename. Rules are evaluated before built-in heuristics, allowing you to
override or extend detection.

Example:

```yaml
classifier:
  rules:
    - regex: "^special.*\.bin$"
      type: "llm.gguf"
```


