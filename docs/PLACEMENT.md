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

Presets
- `comfyui`: ComfyUI `models/checkpoints`, `models/loras`, `models/vae`, `models/controlnet`, and `models/embeddings`.
- `automatic1111`: AUTOMATIC1111 `models/Stable-diffusion`, `models/Lora`, `models/VAE`, `extensions/sd-webui-controlnet/models`, and `embeddings`.
- `forge`: Forge `models/Stable-diffusion`, `models/Lora`, `models/VAE`, `extensions/sd-webui-controlnet/models`, and `embeddings`.
- `ollama`: Ollama `~/.ollama/models` for LLM artifacts.
- `hf-cache`: generic Hugging Face-style export folder under `~/.cache/huggingface/modfetch`.

List presets:
```bash
modfetch place --list-presets
```

Preview a preset without writing a config first:
```bash
modfetch place --path /path/to/model.gguf --preset ollama --dry-run
```

Apply one or more presets on top of an existing config:
```bash
modfetch place --config ./config.yml --path /path/to/model.safetensors --preset comfyui,automatic1111 --dry-run
```

Examples
- Dry-run: see where a file would be placed without making changes:
```
modfetch place --config ./config.yml --path /path/to/file.safetensors --dry-run
```
- Place a downloaded file using detection:
```
modfetch place --config ./config.yml --path /path/to/file.safetensors
```
- Place as LoRA explicitly to ComfyUI and A1111 targets (if mapping configured):
```
modfetch place --config ./config.yml --path /path/to/myLoRA.safetensors --type sd.lora
```

Dry-run output includes the detected type, confidence, placement mode, and the
exact destination path for each planned link/copy. If no mapping applies,
modfetch prints a skip reason instead of silently doing nothing.
