# Batch downloads (jobs.yml)

modfetch can execute multiple downloads from a YAML file via the --batch flag on the download command. Batch mode supports direct HTTP(S) URLs as well as resolver URIs (hf:// and civitai://), optional SHA256 verification, post-download extraction, scheduling windows, and post-download placement.

Key properties:
- YAML-driven; versioned schema (current version: 1)
- No secrets in YAML; tokens must be provided via environment variables (HF_TOKEN, CIVITAI_TOKEN) when the corresponding sources are enabled in config
- Per-job or global placement control
- Per-job archive extraction for `.zip`, `.tar`, `.tar.gz`, and `.tgz`
- Optional local daily schedule windows for jobs

## Schema (version 1)
Top-level:
- version: integer (required) — must be 1
- jobs: array of BatchJob (required)

BatchJob fields:
- uri: string (required)
  - Direct HTTP(S) or resolver URI
  - Examples: `https://...`, `hf://owner/repo/path?rev=main`, `civitai://model/123456?version=42&file=vae`
- mirrors: array of strings (optional)
  - Ordered fallback direct HTTP(S) URLs or resolver URIs. The primary `uri` is tried first; mirrors are tried in listed order using the same destination and SHA256 expectation.
- priority: integer (optional; default 0)
  - Higher-priority jobs are enqueued first. Jobs with the same priority keep their file order.
- dest: string (optional)
  - Destination file path. If omitted:
    - For civitai:// URIs: modfetch saves to `<general.download_root>/<ModelName> - <OriginalFileName>` (sanitized), with collision-safe suffixes.
    - For other URIs: modfetch saves to `<general.download_root>/<basename-of-final-URL>` with query/fragment removed and the name sanitized. When possible (e.g., CivitAI direct endpoints), a HEAD request is used to honor server-provided filenames via Content-Disposition.
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
- extract: boolean (optional; default false)
  - Extract the downloaded file after it completes. Supported formats are `.zip`, `.tar`, `.tar.gz`, and `.tgz`. `.7z` currently returns a clear unsupported-format error.
- extract_dir: string (optional)
  - Directory for extracted files. If omitted, modfetch uses the archive path without its extension.
- schedule_window: string (optional)
  - Local daily time window in `HH:MM-HH:MM` form. Jobs outside the window wait until the next start time. Overnight windows such as `22:00-02:00` are supported.

## Example
```yaml path=null start=null
version: 1
jobs:
  - uri: "https://proof.ovh.net/files/1Mb.dat"
    priority: 10
    mirrors:
      - "https://proof.ovh.net/files/1Mb.dat"
    place: false

  - uri: "https://example.com/models/archive.zip"
    extract: true
    extract_dir: "/models/extracted/archive"
    schedule_window: "22:00-06:00"

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

- You can override default resolver naming per import with `--naming-pattern "..."` (applies when dest is omitted).
  - CivitAI tokens: `{model_name}`, `{version_name}`, `{version_id}`, `{file_name}`, `{file_type}`
  - Hugging Face tokens: `{owner}`, `{repo}`, `{path}`, `{rev}`, `{file_name}`
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
- Global `--extract` causes all jobs to be extracted, but per-job `extract: true` also works.
- Tokens for gated resources must come from environment variables (HF_TOKEN, CIVITAI_TOKEN). Do not put secrets in YAML.
- Chunked downloads are used when supported; otherwise modfetch falls back to single-stream and resumes via `.part` files.
- On re-run, downloads will resume and verify integrity; placement dedupes by SHA256 if `allow_overwrite: false`.
