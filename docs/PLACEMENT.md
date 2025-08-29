# Placement guide

`placement` controls where artifacts are stored for your tools (ComfyUI, Automatic1111, Ollama, LM Studio, vLLM, etc.).

- `apps`: base directories per application and named relative paths (keys)
- `mapping`: rules from detected artifact type → list of targets `{app, path_key}`
- Modes:
  - `symlink`: create symlinks from central download to app dirs
  - `hardlink`: create hardlinks (fails across filesystems)
  - `copy`: copy the file

Deduplication
- If `allow_overwrite: false` and a destination file exists, the placer compares SHA256 and skips if identical; otherwise it errors to prevent unintentional overwrites.

Artifact types (detected heuristically; override with `--type`)
- `sd.checkpoint`, `sd.lora`, `sd.vae`, `sd.controlnet`, `sd.embedding`, `llm.gguf`, `llm.safetensors`, `generic`.

Examples
- Place a downloaded file using detection:
```
modfetch place --config ./config.yml --path /path/to/file.safetensors
```
- Place as LoRA explicitly to ComfyUI and A1111 targets (if mapping configured):
```
modfetch place --config ./config.yml --path /path/to/myLoRA.safetensors --type sd.lora
```

