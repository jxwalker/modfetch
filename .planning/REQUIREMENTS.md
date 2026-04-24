# Requirements: modfetch

**Defined:** 2026-04-23
**Core Value:** A user can fetch the right model artifact for their machine and runtime with high confidence in integrity, low disk waste, and clear operational control.

## v1 Requirements

### State and Finalization Correctness

- [ ] **STATE-01**: A download that has finalized a correct file is persisted as a terminal success state instead of remaining `running`.
- [ ] **STATE-02**: Finalization updates state, progress views, and process cleanup atomically enough that successful jobs do not get stranded in intermediate bookkeeping states.

### Snapshot Planning

- [ ] **SNAP-01**: User can plan a full Hugging Face repo snapshot as one logical artifact.
- [ ] **SNAP-02**: Planning output reports total bytes, file count, shard count, largest file, and estimated staging overhead before download.
- [ ] **SNAP-03**: The tool distinguishes repo snapshots, variants, and shard files in both human and JSON output.
- [ ] **SNAP-04**: User can execute a repo snapshot with one command instead of building a batch manually.

### Integrity and Repair

- [ ] **INTEG-01**: A completed snapshot is verified against an expected manifest of required files.
- [ ] **INTEG-02**: Truncated, undersized, zero-byte, or structurally invalid safetensors shards are not marked complete.
- [ ] **INTEG-03**: User can run repo-level verification after download using a dedicated command.
- [ ] **INTEG-04**: User can repair only failed shards without refetching a healthy repo.
- [ ] **INTEG-05**: Verification results are visible in CLI output and JSON output.

### Storage and Placement

- [ ] **PLAC-01**: Same-filesystem workflows can finalize without an unnecessary second full copy.
- [ ] **PLAC-02**: Finalization reports whether files were renamed, hardlinked, reflinked, symlinked, or copied.
- [ ] **PLAC-03**: Planning output shows expected disk overhead for the chosen placement strategy.
- [ ] **PLAC-04**: Shared cache and runtime placement workflows remain compatible with existing placement rules.

### Runtime Guidance

- [ ] **RUN-01**: User can plan or fetch using runtime-aware presets starting with `vllm`.
- [ ] **RUN-02**: Runtime-aware planning explains why each file is included or excluded.
- [ ] **RUN-03**: `llama.cpp` and `transformers` are supported after `vllm` with runtime-appropriate file selection.
- [ ] **RUN-04**: Prepared output can be handed directly to the selected runtime without manual repo surgery.

### Beginner Workflow

- [ ] **GUIDE-01**: User can check whether a model fits their machine before downloading.
- [ ] **GUIDE-02**: User can run a guided `prepare` flow that chooses or confirms a sane setup path.
- [ ] **GUIDE-03**: Machine inspection reports OS, architecture, memory, relevant accelerator info, and free disk by mount where available.
- [ ] **GUIDE-04**: Beginner-facing output recommends likely-fit model/runtime combinations and explains caveats.

### Catalog and Discovery

- [ ] **CAT-01**: User can search by goal or friendly alias instead of only raw repo IDs.
- [ ] **CAT-02**: The tool can generate next-step launch templates for supported runtimes.

### CivitAI Reliability

- [ ] **CIV-01**: CivitAI downloads survive interrupted sessions without silent corruption.
- [ ] **CIV-02**: Diagnostics distinguish auth issues, provider-side interruptions, network instability, and anti-bot failures.
- [ ] **CIV-03**: Reliability mode remains a first-class workflow, not a side feature.

### Device Profiles and Workflow Packs

- [ ] **PROF-01**: User can target named device profiles that encode runtime and storage defaults.
- [ ] **PROF-02**: The same model request can produce different correct plans for different targets.
- [ ] **PACK-01**: Workflow packs can resolve multiple related artifacts for common workflows.
- [ ] **PACK-02**: Image-model ecosystems can place assets into app-aware destinations in later phases without rewriting core planning.

## v2 Requirements

### Conversion and Advanced Ecosystem Support

- **V2-01**: The tool can execute safe format conversions where a target-native artifact is unavailable.
- **V2-02**: The tool can support broader app-aware image workflow orchestration beyond the initial pack system.

## Out of Scope

| Feature | Reason |
|---------|--------|
| Generic binary mirroring for non-model workloads | Dilutes the product away from model acquisition and runtime preparation |
| Automatic serving lifecycle management | Deployment/orchestration is downstream of acquisition |
| Automatic conversion in the first milestone | Too much risk before manifest, integrity, and placement foundations are solid |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| STATE-01 | Phase 1 | Pending |
| STATE-02 | Phase 1 | Pending |
| SNAP-01 | Phase 1 | Pending |
| SNAP-02 | Phase 1 | Pending |
| SNAP-03 | Phase 1 | Pending |
| SNAP-04 | Phase 1 | Pending |
| INTEG-01 | Phase 2 | Pending |
| INTEG-02 | Phase 2 | Pending |
| INTEG-03 | Phase 2 | Pending |
| INTEG-04 | Phase 2 | Pending |
| INTEG-05 | Phase 2 | Pending |
| PLAC-01 | Phase 3 | Pending |
| PLAC-02 | Phase 3 | Pending |
| PLAC-03 | Phase 3 | Pending |
| PLAC-04 | Phase 3 | Pending |
| RUN-01 | Phase 4 | Pending |
| RUN-02 | Phase 4 | Pending |
| RUN-03 | Phase 4 | Pending |
| RUN-04 | Phase 4 | Pending |
| GUIDE-01 | Phase 5 | Pending |
| GUIDE-02 | Phase 5 | Pending |
| GUIDE-03 | Phase 5 | Pending |
| GUIDE-04 | Phase 5 | Pending |
| CAT-01 | Phase 6 | Pending |
| CAT-02 | Phase 6 | Pending |
| CIV-01 | Phase 7 | Pending |
| CIV-02 | Phase 7 | Pending |
| CIV-03 | Phase 7 | Pending |
| PROF-01 | Phase 8 | Pending |
| PROF-02 | Phase 8 | Pending |
| PACK-01 | Phase 8 | Pending |
| PACK-02 | Phase 8 | Pending |

**Coverage:**
- v1 requirements: 33 total
- Mapped to phases: 33
- Unmapped: 0

---
*Requirements defined: 2026-04-23*
*Last updated: 2026-04-23 after initial roadmap synthesis from the user roadmap and live codebase review*
