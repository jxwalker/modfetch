# Resolvers: hf:// and civitai://

This project supports two URI schemes that resolve to HTTP(S) download URLs with optional Authorization headers.

Supported schemes
- hf:// for Hugging Face repositories
- civitai:// for CivitAI models and versions

Hugging Face (hf://)
- Format 1: hf://{repo}/{path}
- Format 2: hf://{owner}/{repo}/{path}
- Optional query: ?rev={branch-or-commit}, defaults to main
- Example:
  - hf://gpt2/README.md?rev=main
  - hf://openai/whisper/README.md?rev=v1.0
- Resolution:
  - Resolves to https://huggingface.co/{repo-or-owner/repo}/resolve/{rev}/{path}
- Authentication:
  - If sources.huggingface.enabled is true and sources.huggingface.token_env points to an environment variable that is set (e.g., HF_TOKEN), an Authorization: Bearer <token> header is attached.

CivitAI (civitai://)
- Primary format: civitai://model/{modelId}
  - Optional query params:
    - version={versionId}
    - file={substring} (case-insensitive substring match against file name)
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

Examples
- civitai://model/123456            # latest version, primary file
- civitai://model/123456?file=vae   # choose a VAE by name substring
- civitai://model/123456?version=42 # choose a specific version id

Notes
- Tokens must be provided via environment variables. Do not hardcode secrets in YAML.
- The chunked downloader uses these headers for HEAD and ranged GET requests.
- For testing only, an internal override is available to swap the CivitAI API base URL.

