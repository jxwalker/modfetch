# Resolvers: hf:// and civitai://

modfetch resolver URIs turn model registry references into authenticated HTTP(S) downloads. Resolvers also provide metadata used for default filenames and naming patterns.

## Supported schemes

- Hugging Face: `hf://{repo-alias}[/{root-file}]?rev=main[&quant=Q4_K_M]` or `hf://{owner}/{repo}[/{path}]?rev=main[&quant=Q4_K_M]`
- CivitAI: `civitai://model/{id}[?version=ID][&file=substring]`

## Hugging Face

Supported forms:

```text
hf://repo-alias?rev=REVISION
hf://repo-alias/ROOT_FILE?rev=REVISION
hf://owner/repo/path/to/file?rev=REVISION
hf://owner/repo?rev=REVISION&quant=QUANTIZATION
```

Examples:

```bash
modfetch download --url 'hf://gpt2/README.md?rev=main'
modfetch download --url 'hf://bigscience/bloom/LICENSE?rev=main'
modfetch download --url 'hf://openai/whisper/README.md?rev=v1.0'
modfetch download --url 'hf://owner/repo?rev=main&quant=Q4_K_M'
```

Notes:

- `rev` is optional and defaults to `main`.
- `quant` is optional. When set, modfetch selects a matching Hugging Face quantized artifact if one is available.
- Single-name public repositories such as `gpt2` are supported for repo-only URIs and root-level files such as `hf://gpt2/README.md`.
- Nested paths require the explicit namespaced form, `hf://owner/repo/path/to/file`.
- Namespaced repositories such as `owner/repo` are supported, including dotted repo names.
- If `sources.huggingface.enabled` is true and the configured token environment variable is set, modfetch attaches an `Authorization: Bearer <token>` header.

## CivitAI

Supported forms:

```text
civitai://model/MODEL_ID
civitai://model/MODEL_ID?version=VERSION_ID
civitai://model/MODEL_ID?file=FILENAME_SUBSTRING
```

Behavior:

- If `version` is omitted, modfetch selects the latest model version.
- If `file` is provided, modfetch selects the first file whose name contains the substring.
- Otherwise, modfetch picks the primary file, then the first model-type file, then the first file.
- If `sources.civitai.enabled` is true and the configured token environment variable is set, modfetch attaches an `Authorization: Bearer <token>` header.

## Naming

Resolvers populate metadata so default filenames and naming patterns can be expanded accurately.

Configurable pattern keys:

- `sources.huggingface.naming.pattern`
- `sources.civitai.naming.pattern`

Available tokens:

- Hugging Face: `{owner}`, `{repo}`, `{path}`, `{rev}`, `{file_name}`, `{quantization}`
- CivitAI: `{model_name}`, `{version_name}`, `{version_id}`, `{file_name}`, `{file_type}`

Example:

```yaml
sources:
  civitai:
    enabled: true
    token_env: CIVITAI_TOKEN
    naming:
      pattern: "{model_name} - {file_name}"
  huggingface:
    enabled: true
    token_env: HF_TOKEN
    naming:
      pattern: "{repo} - {file_name}"
```

Per-command override:

```bash
modfetch download --config ~/.config/modfetch/config.yml \
  --url 'hf://bigscience/bloom/LICENSE?rev=main' \
  --naming-pattern '{repo} - {file_name}'
```

Final filenames are sanitized and de-duplicated; collision-safe suffixes are added when needed.

For direct HTTP(S) URLs, the default filename is the final URL basename with query and fragment removed. For CivitAI direct download endpoints, the TUI and importer attempt a HEAD request and use a `Content-Disposition` filename when the server provides one.
