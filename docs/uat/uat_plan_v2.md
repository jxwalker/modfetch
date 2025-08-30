# Modfetch TUI v2 – Comprehensive UAT Plan

Audience: Novice-friendly checklist to validate functionality, performance, and integration of modfetch v2 (CLI and TUI), including the new batch import feature.

Environment
- OS: macOS
- Shell: zsh 5.9
- Project dir: /Users/James/Documents/Code/modfetch
- Ensure Go toolchain is installed (go 1.21+ recommended)
- Set MODFETCH_CONFIG to your YAML config path if not passing --config per command

Pre-UAT Preparation
1) Build
   - Run: make build or go build ./...
   - Expect: build succeeds with no errors.
2) Config sanity
   - Create or validate a config. Example minimal config:
     - general:
       - download_root: /Users/James/Downloads/modfetch
       - stage_partials: true
     - sources:
       - huggingface:
         - enabled: true
         - token_env: HF_TOKEN
       - civitai:
         - enabled: true
         - token_env: CIVITAI_TOKEN
     - ui:
       - refresh_hz: 5
       - column_mode: dest
       - compact: false
   - Validate: modfetch config validate --config=<path>
   - Inspect: modfetch config print --config=<path>
3) Tokens (optional but recommended)
   - Export HF_TOKEN and CIVITAI_TOKEN (do not print them). Example:
     - export HF_TOKEN=...; export CIVITAI_TOKEN=...

Section A – CLI Basics
A1) Direct download (public URL)
- Command: modfetch download --config=<cfg> --url=https://speed.hetzner.de/1MB.bin --summary-json
- Expect: JSON summary with dest, size>0, sha present; .sha256 sidecar file created.

A2) Resolver: Hugging Face
- Command: modfetch download --config=<cfg> --url=hf://owner/repo/path/to/file.bin?rev=main
- Expect: Success download; resumes if re-run; verifies SHA post-download.

A3) Resolver: CivitAI
- Command: modfetch download --config=<cfg> --url=civitai://model/<id>?version=<ver>
- Expect: Uses SuggestedFilename for dest when not provided; succeeds with token if required.

A4) Placer
- Command: modfetch place --config=<cfg> --path=<downloaded_file> --dry-run
- Expect: Lists target paths. Then run without --dry-run to place. Verify symlink/copy behavior per config.

A5) Verify
- Command: modfetch verify --config=<cfg> --path=<downloaded_file>
- Expect: “ok” for correct SHA.

Section B – New Feature: Batch Import from Text File
B1) Prepare an input file
- Create input_urls.txt with lines (one per entry). Supported formats:
  - Direct URL
  - hf:// and civitai:// URIs
  - Optional key-value pairs after URL: dest=... sha256=... type=... place=true mode=symlink
  - CivitAI model page URLs (e.g., https://civitai.com/models/<id>?version=...) are auto-normalized to civitai:// unless --no-resolve-pages is used.
  Example lines:
  - https://speed.hetzner.de/1MB.bin
  - hf://owner/repo/path/to/file.bin?rev=main
  - civitai://model/12345?version=67890
  - https://civitai.com/models/12345?modelVersionId=67890
  - https://speed.hetzner.de/10MB.bin dest=/Users/James/Downloads/modfetch/custom.bin type=model place=true mode=symlink

B2) Import
- Command:
  - modfetch batch import \
    --config=<cfg> \
    --input=input_urls.txt \
    --output=batch.yaml \
    --dest-dir=/Users/James/Downloads/modfetch \
    --sha-mode=none
- Expect:
  - batch.yaml is written with version: 1 and a jobs list, each with uri, dest, sha256 (empty if none), type, place, mode.
  - Destinations are inferred when not provided (Content-Disposition, final URL path, or resolver-suggested), uniquified to avoid conflicts.

B3) Optional SHA computation
- Command: same as above but add --sha-mode=compute (this downloads content to compute hashes; expect it to be slow and bandwidth-heavy).
- Expect: batch.yaml includes sha256 for entries (may require auth tokens).

B4) Batch download
- Command: modfetch download --config=<cfg> --batch=batch.yaml --summary-json
- Expect: All jobs download; JSON summaries print; .sha256 files are written; placing occurs if place=true or --place flag provided.

B5) Failure handling
- Temporarily set an invalid URL in input file, re-import and download.
- Expect: Affected job logs an error; others proceed.

Section C – TUI v2 Functional UAT
C1) Launch TUI
- Command: modfetch tui --config=<cfg>
- Expect: TUI shows header, footer, tabs, and a table.

C2) Filtering
- Press / to toggle filter; type a substring of URL or dest; press Enter to apply; Esc to clear.
- Expect: Table filters to matching rows.

C3) Sorting
- Press s to sort by speed; e to sort by ETA; o to clear sorting.
- Expect: Visible rows reorder appropriately.

C4) Grouping
- Press g to group by host; press again to ungroup.
- Expect: Rows grouped, headers show host groups.

C5) Selection
- Navigate with j/k or arrow keys; Space toggles selection; A selects all visible; X clears selection.
- Expect: Selected rows visually distinct; retry/cancel actions apply to all selected or current when none selected.

C6) Toasts and toast drawer
- Trigger actions (e.g., start/stop downloads) and observe inline toasts.
- Press H to open/close toast drawer with timestamps.

C7) Help modal
- Press ? to toggle; verify it lists all keybindings.

C8) Theme presets
- Press T repeatedly; cycle through presets (Base, Neon, Dracula, Solarized); ensure persistence on quit/restart.

C9) Last column cycle
- Press t to cycle between dest → url → host; ensure it persists.

C10) Compact mode
- Press v to toggle; hides speed/throughput; ensure it persists.

C11) Refresh rate
- Set ui.refresh_hz in config (0..10); verify ticker behavior (smoother updates at higher Hz).

C12) State persistence
- Change theme, column mode, grouping, sorting; quit; relaunch TUI; verify settings are restored from data_root/ui_state_v2.json.

Section D – Performance and Stability
D1) Rendering performance
- Start 10+ concurrent downloads (mix of sizes). Verify only visible rows update, offscreen formatting is skipped. No CPU spikes.

D2) Metrics caching
- With many active rows, ensure smooth updates and accurate sparklines.

D3) Toast retention
- Generate >50 toasts; verify UI only retains latest ~50 without leaks.

D4) Resume and retries
- Interrupt a download; re-run; verify resume works and retries are counted (visible in progress line of CLI; in TUI columns if available).

Section E – Integration/Config
E1) Auth headers
- With tokens set, access gated HF/CivitAI assets; verify 401 errors disappear, downloads proceed.

E2) Placement
- Set default placement mode in config; run batch with place=true; verify outputs are placed correctly.

E3) Clean
- Run modfetch clean --config=<cfg> --days=0 --dry-run; verify it lists staged .part files; then run to remove as needed.

Section F – Recording and Docs
F1) Asciinema recording
- Install asciinema; record key scenarios:
  - Filtering, sorting, grouping
  - Multi-select and batch actions
  - Theme cycle and compact mode
  - Toast drawer and help modal
- Save cast files and add to documentation per your docs guidelines.

Troubleshooting Tips
- If you see 401 Unauthorized for HF/CivitAI, ensure tokens are exported and that you’ve accepted license agreements where required.
- If a dest path already exists, importer uniquifies paths (dest (2).ext, etc.). You can override with dest=... per line.
- For SHA compute mode, be aware it fully downloads content; use selectively.

