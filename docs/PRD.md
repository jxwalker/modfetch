# modfetch PRD (Linux dev-server readiness)

Status: Draft for MVP testing on Linux dev server
Owner: James
Last updated: 2025-08-29

## 1. Problem statement
ML practitioners and hobbyists frequently download large model assets (LLMs, Stable Diffusion models, LoRAs, VAEs) from multiple sources.
Manual downloads are slow and error-prone (network hiccups, broken resumes, checksum mismatches), and organizing assets across
apps (ComfyUI, A1111, Ollama, LM Studio, vLLM) is tedious. We need a robust CLI/TUI tool that reliably downloads, verifies,
and places artifacts with observability and resumability.

## 2. Goals
- Reliable, resumable downloads with integrity verification
  - Parallel chunked downloads with retry/backoff
  - Single-stream fallback for servers without Range/HEAD
  - SHA256 per-chunk (for repair) and final file, plus .sha256 sidecar
- Unified sources
  - Direct HTTP(S)
  - Resolvers: hf:// (Hugging Face), civitai:// (CivitAI) with token-based auth via env vars
- Organization and placement
  - Placement engine to symlink/hardlink/copy into app directories per config mapping
  - Safe dedupe and overwrite control via checksum comparison
- State, observability, and UX
  - SQLite state DB for downloads/chunks with statuses
  - Log levels (human/JSON) and Prometheus textfile metrics
  - CLI progress and TUI dashboard
- Batch execution from YAML with optional placement

## 3. Non-goals (MVP)
- Background daemon/service mode
- Distributed/multi-host coordination or remote workers
- Advanced queueing/scheduling/prioritization beyond simple concurrency
- Artifact signing or provenance verification beyond SHA256
- Additional URI schemes (e.g., s3://) — may come later via plugins

## 4. Personas
- Linux Dev/Ops: Sets up and validates the tool on Ubuntu servers, integrates with existing workflows and Prometheus.
- ML Researcher/Hobbyist: Uses CLI/TUI to fetch and manage models locally.

## 5. User journeys (happy paths)
- Setup on Ubuntu
  1) Install Go 1.22+ and build the binary (make build)
  2) Create config YAML; export HF_TOKEN/CIVITAI_TOKEN if needed
  3) Run smoke tests: a small HTTP and hf:// download, verify, and TUI
- Single download
  - modfetch download --url 'hf://gpt2/README.md?rev=main' — shows progress; writes .sha256; status/verify confirm
- Batch run
  - modfetch download --batch jobs.yml --place — executes jobs; places into configured app directories
- TUI monitoring
  - modfetch tui — monitors status/progress with filter and details

## 6. Functional requirements
- Configuration
  - YAML-driven; ~ expansion; no secrets stored in YAML
  - Token env vars: HF_TOKEN, CIVITAI_TOKEN (only if sources enabled)
  - Command flags: --config, --log-level, --json (+ download-specific flags)
- Resolvers
  - hf://owner/repo/path?rev=branch — resolves to https://huggingface.co/{owner/repo}/resolve/{rev}/{path}
  - civitai://model/{id}?version={id}&file={substring} — fetches model/version, selects appropriate file
  - If token env present and enabled, attach Authorization: Bearer <token>
- Downloaders
  - HEAD to detect size and Range support
  - Chunk planning using configured chunk size and concurrency; retries with jitter backoff
  - Persist chunk state (pending/running/complete/dirty); after final hash mismatch, scan chunks and repair mismatched ones
  - Fall back to single-stream when necessary; resume from .part
  - On success: rename .part → final, emit .sha256, update state to complete
- Placement
  - Derive artifact type (or override via --type) and compute targets from mapping
  - Modes: symlink/hardlink/copy; safe dedupe (skip identical by SHA256), respect allow_overwrite
- Status & verify
  - status: list downloads (JSON or table); verify: recompute SHA256 and update status (verified or mismatch)
- TUI
  - Keys: q, r, j/k, d, /; compute progress from chunks or .part file
- Metrics & logging
  - Prometheus textfile metrics (bytes, retries, success count, last download duration, timestamp)
  - Log levels: debug/info/warn/error; JSON optional

## 7. Non-functional requirements
- Performance: achieve 70–80% of available bandwidth on a typical 1 Gbps link for large files
- Reliability: resumable; chunk repair on mismatch; stable under transient HTTP failures
- Portability: Linux primary, macOS secondary; CGO disabled for releases
- Security: no secrets in YAML; tokens via env; avoid logging secrets

## 8. Configuration constraints
- All secrets must be provided via environment variables
- Paths may contain ~ and are expanded to absolute paths

## 9. Acceptance criteria (UAT)
Aligned with docs/TESTING.md; minimally:
- Reliability
  - Resume works (kill mid-download and rerun)
  - Chunk repair works on tampered .part; final SHA256 matches expected
  - Works against Range and non-Range servers (fallback tested)
- Functionality
  - hf:// and civitai:// resolve and download as expected (with tokens for gated content)
  - Placement performs symlink/hardlink/copy modes correctly per mapping
  - Batch with --place completes successfully
- Observability
  - Metrics are written to configured textfile path
  - Logs are informative; JSON mode works; quiet hides progress
- Performance
  - Throughput hits acceptable targets given network conditions

## 10. Out-of-scope checks
- No background service or auto-start at boot
- No plugin schemes beyond hf:// and civitai:// in MVP

## 11. Rollout plan
- Validate locally on Linux dev server following docs/DEPLOY_LINUX.md
- Add CI job to run `go test ./...` and optional static checks
- Tag v0.x for release; attach artifacts via existing release workflow

## 12. Risks & mitigations
- Server-side API/rate limits → backoff & retries; allow user-provided tokens
- Filesystem differences (hardlink across FS) → fallback to copy
- Large-file integrity → per-chunk and final SHA256; repair flow; .sha256 sidecar

## 13. Open questions
- Target Linux distro(s) and minimum kernel/glibc baseline?
- Default placement mapping presets for common apps (ComfyUI/A1111) — which ones to ship?
- Metrics pipeline expectations (Prometheus textfile vs nothing for MVP)?
- Any need for systemd user units or just tmux guidance?

