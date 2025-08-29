# TODO / Plan: CivitAI model-aware default filenames

Goal
- When downloading via civitai:// and no --dest is provided, default the destination filename to include the CivitAI model name so files are easy to recognize and organize.

Default behavior
- Pattern: "<ModelName> - <OriginalFileName>"
  - Keep the original extension (it comes from CivitAI’s file name).
  - Sanitize the final filename (replace path separators and disallowed chars with underscores).
- Scope: Only applies when the user provides civitai:// URIs and omits --dest (and dest is empty in batch jobs). Direct https://civitai.com URLs retain the current behavior.
- Collision policy: If the computed filename already exists under download_root, try adding version ID hint " (v<versionId>)"; if still collides, add numeric suffixes before the extension: " (2)", "(3)", etc.

Implementation tasks
1) Extend Resolved metadata
   - internal/resolver/resolver.go: add optional fields to Resolved: ModelName, VersionName, VersionID, FileName, SuggestedFilename.
   - HuggingFace resolver leaves these empty; CivitAI resolver populates them.

2) Populate CivitAI metadata and suggested filename
   - internal/resolver/civitai.go:
     - Extend API structs to capture model/version names (e.g., civitModel.Name, civitVersion.Name, civitVersion.ModelID).
     - If ?version is present, fetch version and (if needed) fetch model to obtain model name; else fetch model (contains versions) and pick latest version as today.
     - Preserve existing file selection logic (file substring, primary, Model type, fallback first).
     - After selecting a file, set: ModelName, VersionName, VersionID, FileName (file.Name).
     - Compute SuggestedFilename = SafeFileName(ModelName + " - " + FileName).

3) Shared helpers
   - internal/util/paths.go (new):
     - SafeFileName(name string) string — centralize filename sanitization.
     - UniquePath(dir, base, versionHint string) (string, error) — returns a unique filename in dir, applying version hint and numeric suffixes.
   - internal/downloader/chunked.go: switch to util.SafeFileName and remove duplicated local helper.

4) Wire CLI to use SuggestedFilename
   - cmd/modfetch/main.go:
     - After resolver.Resolve for civitai://, when dest is empty and res.SuggestedFilename is set, compute dest = UniquePath(download_root, res.SuggestedFilename, res.VersionID).
     - Use this dest for the progress display (so the .part path matches) and for the downloader.
   - Batch mode: same behavior for jobs with empty dest.

5) Tests
   - internal/resolver/civitai_test.go: assert that resolver populates ModelName, VersionName, SuggestedFilename and still chooses the correct file.
   - internal/util/paths_test.go: SafeFileName and UniquePath edge cases (illegal chars, collisions, suffix placement before extension).
   - Update any outdated tests (e.g., downloader resolve tests) to reflect signatures and new behavior.

6) Docs
   - docs/RESOLVERS.md, docs/BATCH.md, README.md: document civitai default naming when dest is omitted, scope/limits, and collision policy.

Acceptance criteria
- Given civitai://model/{id} (with or without ?version) and no --dest, the final saved filename contains the model name following the pattern above and retains the correct extension.
- When the computed name collides in download_root, a unique name is chosen via version hint or numeric suffix.
- Progress bar reflects the chosen destination path from start to finish.
- Behavior for hf:// and direct HTTP(S) downloads remains unchanged.
- Unit tests for resolver metadata and helpers pass; existing tests remain green.
- Docs updated to reflect the new behavior.

Out of scope (for now)
- Config/CLI knobs for naming patterns; can be added later if needed (proposal: sources.civitai.naming.pattern, include_version: bool).
- Deriving model names for direct https://civitai.com links (non-civitai:// URIs).

