# Resolvers: hf:// and civitai://

This project supports two URI schemes that resolve to HTTP(S) download URLs with optional Authorization headers.

Supported schemes
- hf:// for Hugging Face repositories
- civitai:// for CivitAI models and versions

Resolver matrix and examples
- hf://{repo}/{path}
  - Example: hf://gpt2/README.md?rev=main → https://huggingface.co/gpt2/resolve/main/README.md
- hf://{owner}/{repo}/{path}
  - Example: hf://openai/whisper/README.md?rev=v1.0 → https://huggingface.co/openai/whisper/resolve/v1.0/README.md
- civitai://model/{modelId}
  - Example: civitai://model/123456 → selects latest version’s primary file
- civitai://model/{modelId}?version={versionId}
  - Example: civitai://model/123456?version=42 → selects specific version
- civitai://model/{modelId}?file={substring}
  - Example: civitai://model/123456?file=vae → picks first file name containing “vae”

Hugging Face (hf://)
- Optional query: ?rev={branch-or-commit}, defaults to main
- Resolution:
  - Resolves to https://huggingface.co/{repo-or-owner/repo}/resolve/{rev}/{path}
- Authentication:
  - If sources.huggingface.enabled is true and sources.huggingface.token_env points to an environment variable that is set (e.g., HF_TOKEN), an Authorization: Bearer <token> header is attached.

CivitAI (civitai://)
- Behavior:
  - If version is omitted, the latest version (by version id) is selected.
  - If file is provided, the first file whose name contains the substring is selected.
  - Otherwise, picks the primary file, or first Model-type file, then fallback to first file.
- Resolution:
  - The resolver queries the CivitAI API:
    - GET /api/v1/models/{modelId} to enumerate versions and files
    - or GET /api/v1/model-versions/{versionId}
  - Returns the file.downloadUrl from the selected file.
- Authentication:
  - If sources.civitai.enabled is true and sources.civitai.token_env points to an environment variable (e.g., CIVITAI_TOKEN), an Authorization: Bearer <token> header is attached and used for both HEAD and GET requests.

Notes
- Tokens must be provided via environment variables. Do not hardcode secrets in YAML.
- The chunked downloader uses these headers for HEAD and ranged GET requests.
- For testing only, an internal override is available to swap the CivitAI API base URL.

