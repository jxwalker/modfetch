# Roadmap: modfetch

## Overview

`modfetch` already has strong download primitives. The roadmap now shifts the product from a capable downloader into a model-centric acquisition system that beats `hf downloader` and `aria2` by planning full model snapshots, guaranteeing integrity, minimizing disk waste, and guiding both expert operators and beginners toward working local setups.

## Phases

- [ ] **Phase 1: First-Class HF Snapshots and Manifest Planner** - Build manifest-driven Hugging Face snapshot planning and execution.
- [ ] **Phase 2: Repo-Level Verification and Shard Repair** - Make integrity a first-class repo workflow instead of a file-level afterthought.
- [ ] **Phase 3: Efficient Finalization and Deduplicated Placement** - Remove avoidable duplicate copies and make storage strategy explicit.
- [ ] **Phase 4: Runtime-Aware Fetching and Presets** - Add runtime-aware planning starting with `vllm`, then expand to `llama.cpp` and `transformers`.
- [ ] **Phase 5: Machine Detection and Guided Prepare Flows** - Add `check` and `prepare` so beginners can get a compatible setup without deep model knowledge.
- [ ] **Phase 6: Catalog, Aliases, and Launch Templates** - Make model discovery and next-step launch guidance friendlier and more approachable.
- [ ] **Phase 7: Hardened CivitAI Reliability Mode** - Strengthen CivitAI resume, recovery, diagnostics, and corruption handling under hostile conditions.
- [ ] **Phase 8: Device Profiles, Workflow Packs, and Image Ecosystem Support** - Add target-aware profiles and multi-artifact workflow packs for mixed fleets and image workflows.

## Phase Details

### Phase 1: First-Class HF Snapshots and Manifest Planner
**Goal**: Fix terminal-state bookkeeping first, then convert Hugging Face handling from file-centric resolution to repo-centric snapshot manifests with usable planning output.
**Depends on**: Nothing (first phase)
**Requirements**: [STATE-01, STATE-02, SNAP-01, SNAP-02, SNAP-03, SNAP-04]
**Success Criteria** (what must be TRUE):
  1. A successfully finalized download cannot remain stuck in `running` state in SQLite or user-facing status views.
  2. User can run `modfetch plan hf://owner/repo` and see real snapshot-level planning data.
  3. User can run `modfetch snapshot hf://owner/repo` and fetch an entire repo as one logical job.
  4. Output clearly separates repo, variant, and shard concepts instead of conflating them.
  5. The planner becomes the source of truth for later verification and placement work.
**Plans**: 4 plans

Plans:
- [x] 01-01: Reproduce and fix the finalization/state-cleanup bug so successful jobs transition atomically to a terminal success state.
- [ ] 01-02: Add manifest domain types and Hugging Face repo-tree expansion.
- [ ] 01-03: Add `plan` and `snapshot` CLI surfaces with JSON and human output.
- [ ] 01-04: Redesign quant/variant reporting to use manifest semantics instead of selected-file heuristics.

### Phase 2: Repo-Level Verification and Shard Repair
**Goal**: Guarantee that snapshot completion means the repo is structurally sound and repairable at shard granularity.
**Depends on**: Phase 1
**Requirements**: [INTEG-01, INTEG-02, INTEG-03, INTEG-04, INTEG-05]
**Success Criteria** (what must be TRUE):
  1. A repo with a bad shard fails verification clearly and names the exact problem file.
  2. Repo verification checks required files, sizes, and shard/index consistency in addition to existing file checks.
  3. Corrupt or incomplete shards are quarantined or marked suspect instead of accepted as complete.
  4. User can repair only failed shards from manifest diff data.
**Plans**: 3 plans

Plans:
- [ ] 02-01: Persist manifest and verification receipt data in local state.
- [ ] 02-02: Add repo-level `verify` flow for snapshot directories and manifest-backed jobs.
- [ ] 02-03: Add targeted shard repair and suspect-file handling.

### Phase 3: Efficient Finalization and Deduplicated Placement
**Goal**: Make large-model downloads storage-aware so they do not duplicate terabytes unnecessarily.
**Depends on**: Phase 2
**Requirements**: [PLAC-01, PLAC-02, PLAC-03, PLAC-04]
**Success Criteria** (what must be TRUE):
  1. Same-filesystem workflows can finalize without an unnecessary second copy.
  2. Planning output tells the user what overhead to expect before downloading.
  3. Finalization status reports the exact strategy used per artifact.
  4. Existing placement workflows keep working while gaining more efficient options.
**Plans**: 3 plans

Plans:
- [ ] 03-01: Add explicit finalization strategy modeling including rename, hardlink, reflink, symlink, and copy.
- [ ] 03-02: Teach planner and downloader to choose and report the lowest-overhead safe strategy.
- [ ] 03-03: Integrate deduplicated placement with existing placer rules and state reporting.

### Phase 4: Runtime-Aware Fetching and Presets
**Goal**: Teach `modfetch` to choose the right files and layout for supported local runtimes.
**Depends on**: Phase 3
**Requirements**: [RUN-01, RUN-02, RUN-03, RUN-04]
**Success Criteria** (what must be TRUE):
  1. `--runtime vllm` fetches the minimal required set for local serving from supported HF repos.
  2. Planner output explains why each file is included.
  3. `llama.cpp` and `transformers` presets follow after `vllm` without regressing snapshot semantics.
  4. Runtime-ready output can be pointed at the target runtime without manual repo surgery.
**Plans**: 3 plans

Plans:
- [ ] 04-01: Add runtime profile model and `vllm` file-selection rules.
- [ ] 04-02: Add runtime-aware layout presets and explanation output.
- [ ] 04-03: Extend runtime support to `llama.cpp` and `transformers`.

### Phase 5: Machine Detection and Guided Prepare Flows
**Goal**: Make it practical for a beginner to answer “can I run this?” and get a working setup path.
**Depends on**: Phase 4
**Requirements**: [GUIDE-01, GUIDE-02, GUIDE-03, GUIDE-04]
**Success Criteria** (what must be TRUE):
  1. User can run a machine check before downloading.
  2. Beginner output clearly warns about disk, memory, and runtime caveats.
  3. `prepare` can carry a beginner through check → plan → fetch → verify → next-step launch guidance.
  4. Advanced users can still override profile, runtime, and placement decisions explicitly.
**Plans**: 3 plans

Plans:
- [ ] 05-01: Add machine inspection for OS, architecture, memory, storage, and available accelerator signals.
- [ ] 05-02: Add `check` and first-pass recommendation logic.
- [ ] 05-03: Add `prepare` happy path built on planner, verifier, and runtime presets.

### Phase 6: Catalog, Aliases, and Launch Templates
**Goal**: Improve discovery and reduce the amount of model-specific knowledge required to get started.
**Depends on**: Phase 5
**Requirements**: [CAT-01, CAT-02]
**Success Criteria** (what must be TRUE):
  1. User can search by goal or friendly alias instead of only by raw model repo.
  2. The tool can explain what it chose and why.
  3. Launch templates for supported runtimes are emitted automatically as next steps.
**Plans**: 2 plans

Plans:
- [ ] 06-01: Add curated alias/catalog model and lookup commands.
- [ ] 06-02: Add runtime launch templates and explanation output.

### Phase 7: Hardened CivitAI Reliability Mode
**Goal**: Preserve and deepen one of `modfetch`’s original differentiators under hostile network conditions.
**Depends on**: Phase 6
**Requirements**: [CIV-01, CIV-02, CIV-03]
**Success Criteria** (what must be TRUE):
  1. Interrupted CivitAI downloads resume cleanly without silent corruption.
  2. Diagnostic output distinguishes likely cause categories instead of generic failures.
  3. Reliability mode is clearly better than browser-based downloads under flaky conditions.
**Plans**: 2 plans

Plans:
- [ ] 07-01: Add hardened retry/resume behavior and integrity promotion rules for CivitAI.
- [ ] 07-02: Add `doctor civitai` diagnostics and recovery flows.

### Phase 8: Device Profiles, Workflow Packs, and Image Ecosystem Support
**Goal**: Let one request produce different correct outcomes for different machines and multi-artifact workflows.
**Depends on**: Phase 7
**Requirements**: [PROF-01, PROF-02, PACK-01, PACK-02]
**Success Criteria** (what must be TRUE):
  1. Named device profiles encode storage, runtime, and format defaults for heterogeneous fleets.
  2. Workflow packs can resolve multiple related artifacts as one request.
  3. Image-model workflows can evolve on top of the same planning and verification foundations rather than a separate subsystem.
**Plans**: 3 plans

Plans:
- [ ] 08-01: Add target profile model and profile-aware planning.
- [ ] 08-02: Add workflow pack manifests for multi-artifact requests.
- [ ] 08-03: Add first image-model ecosystem adapters on the shared planning core.

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. First-Class HF Snapshots and Manifest Planner | 1/4 | In progress | - |
| 2. Repo-Level Verification and Shard Repair | 0/3 | Not started | - |
| 3. Efficient Finalization and Deduplicated Placement | 0/3 | Not started | - |
| 4. Runtime-Aware Fetching and Presets | 0/3 | Not started | - |
| 5. Machine Detection and Guided Prepare Flows | 0/3 | Not started | - |
| 6. Catalog, Aliases, and Launch Templates | 0/2 | Not started | - |
| 7. Hardened CivitAI Reliability Mode | 0/2 | Not started | - |
| 8. Device Profiles, Workflow Packs, and Image Ecosystem Support | 0/3 | Not started | - |
