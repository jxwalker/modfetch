# Changelog

All notable changes to this project will be documented in this file.

## v0.2.0 - 2025-08-29
- Feature: Model-aware default filenames for CivitAI downloads
  - Resolver returns model/version metadata and SuggestedFilename
  - CivitAI resolver computes `<ModelName> - <OriginalFileName>` (sanitized)
  - CLI uses SuggestedFilename when `--dest` is omitted for `civitai://` URIs
  - Collision handling via `(v<versionId>)` or numeric suffixes `(2)`, `(3)`, etc.
- Utilities: Added `util.SafeFileName` and `util.UniquePath` helpers
- Tests: Updated downloader tests; added tests for util helpers; assertions for SuggestedFilename in resolver tests
- Docs: Updated README.md, docs/RESOLVERS.md, docs/BATCH.md; added docs/TODO.md plan

## v0.1.0 - 2025-08-29
- Initial MVP with:
  - hf:// and civitai:// resolvers
  - Chunked and single-stream downloaders with resume
  - SHA256 verification, state DB, TUI skeleton, placement engine

