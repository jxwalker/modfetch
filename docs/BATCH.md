# Batch downloads (jobs.yml)

modfetch can execute multiple downloads from a YAML file via the --batch flag on the download command. Batch mode supports direct HTTP(S) URLs as well as resolver URIs (hf:// and civitai://), optional SHA256 verification, and post-download placement.

Key properties:
- YAML-driven; versioned schema (current version: 1)
- No secrets in YAML; tokens must be provided via environment variables (HF_TOKEN, CIVITAI_TOKEN) when the corresponding sources are enabled in config
- Per-job or global placement control

## Schema (version 1)
Top-level:
- version: integer (required) — must be 1
- jobs: array of BatchJob (required)

BatchJob fields:
- uri: string (required)
  - Direct HTTP(S) or resolver URI
  - Examples: `https://...`, `hf://owner/repo/path?rev=main`, `civitai://model/123456?version=42&file=vae`
- dest: string (optional)
  - Destination file path. If omitted:
    - For civitai:// URIs: modfetch saves to `<general.download_root>/<ModelName> - <OriginalFileName>` (sanitized), with collision-safe suffixes.
    - For other URIs: modfetch saves to `<general.download_root>/<basename-of-resolved-URL>`.
- sha256: string (optional)
  - Expected final SHA256 (hex). On mismatch, modfetch will re-hash chunks to identify and re-fetch the corrupted ones. If still mismatched, the job fails.
  - A `.sha256` sidecar file is always written on success.
- type: string (optional)
  - Artifact type override used by placement. If omitted, modfetch detects from filename heuristics.
  - Common values: `sd.checkpoint`, `sd.lora`, `sd.vae`, `sd.controlnet`, `sd.embedding`, `llm.gguf`, `llm.safetensors`, `generic`.
- place: boolean (optional; default false)
  - Whether to place the file immediately after download using your placement mapping.
- mode: string (optional)
  - Placement mode override: `symlink` | `hardlink` | `copy`. If omitted, falls back to `general.placement_mode` from your config.

## Example
```yaml path=null start=null
version: 1
jobs:
  - uri: "https://proof.ovh.net/files/1Mb.dat"
    place: false

  - uri: "hf://gpt2/README.md?rev=main"
    # dest: "/absolute/path/optional"
    place: false

  # Requires CIVITAI_TOKEN if the asset is gated; provide a real model/version and file selector for your environment
  # - uri: "civitai://model/123456?version=42&file=vae"
  #   place: true
  #   mode: "symlink"   # symlink | hardlink | copy
```

See also: `assets/sample-config/jobs.example.yml` in this repository.

## Running a batch
- Place your jobs file at a convenient path, e.g., `/opt/modfetch/jobs/uat.yml` or within your repo.
- Execute with your YAML config and optionally enable global placement for all jobs via --place:

```bash path=null start=null
modfetch download \
  --config /path/to/config.yml \
  --batch /path/to/jobs.yml \
  --place
```

Notes:
- Global `--place` causes all jobs to be placed, but per-job `place: true` also works; either will trigger placement.
- Tokens for gated resources must come from environment variables (HF_TOKEN, CIVITAI_TOKEN). Do not put secrets in YAML.
- Chunked downloads are used when supported; otherwise modfetch falls back to single-stream and resumes via `.part` files.
- On re-run, downloads will resume and verify integrity; placement dedupes by SHA256 if `allow_overwrite: false`.

