# modfetch

## What This Is

`modfetch` is a brownfield Go CLI/TUI for downloading and organizing AI model artifacts from Hugging Face, CivitAI, and direct URLs. The next stage is to turn it from a strong downloader into a model-centric acquisition tool that serves both experienced operators who want tight control and beginners who just want the right model to work.

## Core Value

A user can fetch the right model artifact for their machine and runtime with high confidence in integrity, low disk waste, and clear operational control.

## Requirements

### Validated

- ✓ Reliable chunked and single-stream downloads with resume/retry already exist in the codebase.
- ✓ SHA256 verification and deep safetensors validation already exist for file-level workflows.
- ✓ Resolver support for `hf://` and `civitai://` already exists.
- ✓ Placement, TUI monitoring, SQLite-backed state, and library scanning already exist.

### Active

- [ ] Make download finalization and SQLite job state converge reliably so completed artifacts never remain stuck as `running`.
- [ ] Make Hugging Face repos first-class snapshot artifacts instead of file-centric downloads.
- [ ] Add repo-aware integrity verification and targeted shard repair.
- [ ] Minimize duplicate copies across staging, cache, placement, and runtime consumption.
- [ ] Add runtime-aware planning and prepare flows for common local runtimes.
- [ ] Make beginner workflows practical through machine checks, recommendations, aliases, and launch guidance.
- [ ] Preserve and strengthen CivitAI as a core differentiated workflow.

### Out of Scope

- Automatic training, fine-tuning, or serving infrastructure management — not core to acquisition and placement.
- Becoming a generic package manager for arbitrary binaries — the focus remains model and companion-asset workflows.
- Full automatic format conversion in the first milestone — conversion awareness matters now; conversion execution can follow later.

## Context

The repo already has strong download primitives: chunk planning, retries, verification, placement rules, metadata fetchers, a TUI, and local state. The major gap is product shape. Today the tool is still primarily file-oriented, especially for Hugging Face, while large-model users need repo snapshots, runtime-aware file selection, disk-efficient placement, repo-level verification, and recovery. Beginner usability is also limited because the current CLI assumes the user already knows model IDs, formats, runtimes, and storage tradeoffs.

The roadmap provided by the user is grounded in a real operator workflow: large NAS-backed model storage, constrained boot disks, sharded safetensors repos, mixed runtimes, and flaky CivitAI conditions. The product needs to be meaningfully better than `hf` plus `aria2` by owning the planning, integrity, placement, and recovery story end to end.

There is also an immediate correctness bug from a recent real run: the downloaded model file was correct and usable, but `modfetch` left the process and SQLite row in `running` state after final placement completed. That makes terminal-state bookkeeping a prerequisite for trusting later planner and verification layers.

## Constraints

- **Tech stack**: Go codebase with existing CLI/TUI/state architecture — planning should extend current package seams instead of forcing a rewrite.
- **Brownfield**: Existing download and verification behavior must remain stable for current users.
- **Integrity**: “Download complete” must imply trustworthy artifact state, not just bytes written.
- **Efficiency**: Large-model workflows must avoid unnecessary duplicate copies and make overhead visible before execution.
- **UX split**: The same product must satisfy advanced operators and low-context beginners without bifurcating the codebase.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Keep `modfetch` as a brownfield extension of the existing CLI/TUI architecture | The repo already has solid primitives worth preserving | ✓ Good |
| Treat model acquisition as a manifest-driven workflow | Planning, verification, placement, and repair all need a shared source of truth | ✓ Good |
| Fix terminal-state bookkeeping before expanding planner/state responsibilities | Snapshot and verification work depends on trustworthy lifecycle state | ✓ Good |
| Prioritize Hugging Face snapshots before broader beginner/catalog work | The biggest capability gap is repo-centric large-model workflows | ✓ Good |
| Bias the first milestone toward trust and efficiency before convenience layers | Beginner UX is only credible if downloads are correct and storage-aware | ✓ Good |

---
*Last updated: 2026-04-23 after initial GSD planning for the modfetch roadmap*
