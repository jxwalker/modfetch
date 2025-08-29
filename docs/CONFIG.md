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
- `network`
  - `timeout_seconds`, `max_redirects`, `tls_verify`, `user_agent`
- `concurrency`
  - `global_files`, `per_file_chunks`, `chunk_size_mb`, `max_retries`, `backoff`
- `sources`
  - `huggingface`: `enabled`, `token_env`
  - `civitai`: `enabled`, `token_env`
- `placement`
  - `apps`: map of app names → base + relative paths
  - `mapping`: artifact type → list of targets (app/path_key)
- `logging`
  - `level`, `format`, `file`: path and rotation
- `metrics`
  - `prometheus_textfile`: `enabled`, `path`
- `validation`
  - `require_sha256`, `accept_md5_sha1_if_provided`

See `assets/sample-config/config.example.yml` for a full example.

## Tokens
- Set in environment, not in YAML:
  - `export HF_TOKEN=...`
  - `export CIVITAI_TOKEN=...`

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


