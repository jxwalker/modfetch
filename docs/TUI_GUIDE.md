# TUI Guide

This guide walks through launching the TUI, understanding the layout, available actions, and tips for troubleshooting.

## Launching the TUI

- Using a specific config file:

```bash
modfetch tui --config /path/to/config.yml
```

- If you set a default config:

```bash
export MODFETCH_CONFIG=~/.config/modfetch/config.yml
modfetch tui
```

- Preview the next‑gen TUI v2 (experimental):

```bash
modfetch tui --config /path/to/config.yml --v2
```

## Layout overview

- Header: summary, filters, and sort indicators
- Table: one row per download/resolution task
- Footer/help: key hints

Row columns include status, progress, speed, ETA, size, and destination path.

## Row lifecycle

- Resolving: a spinner row appears immediately after starting a download (preflight + resolve)
- Planning: computing chunks and resumable ranges
- Running: chunked or single-stream download with live speed and ETA
- Finalizing: integrity verification, safetensors trim/repair
- Complete or Error: final state stored in the DB

Notes:
- Ephemeral rows are keyed by URL|Dest to avoid collisions and are cleared precisely when the corresponding DB row completes.
- CivitAI model page URLs (https://civitai.com/models/ID) are accepted and auto-resolved to a direct file URL.

## Keybindings

Global
- Navigation: j/k (select), / (filter), m (menu), h or ? (help)
- Sorting: s (by speed), e (by ETA), o (clear sort)
- View/Columns/Theme: v (compact view), t (cycle last column URL/DEST/HOST), T (cycle theme)
- Actions: n (new), r (refresh), d (details), g (group by status)

Per-row
- p (pause/cancel)
- y (retry)
- C (copy path to clipboard)
- U (copy source URL to clipboard)
- O (open file or reveal in file manager)
- D (delete staged data for the row)
- Space (toggle selection), A (select all), X (clear selection)

## Starting new downloads

- Press n to open the new-download modal
- Paste a URL (hf://org/repo/path?rev=... or civitai://model/ID[?file=...]) or a public HTTP/HTTPS URL
- Destination guessing is sanitized and remains under your configured download_root

## Open/Reveal behavior

- The TUI runs the open/reveal command synchronously to catch errors (e.g., missing file manager). Any error is surfaced in the UI.
- On macOS, this is typically `open`; on Linux, `xdg-open`.

## Filtering and grouping

- Press / to filter by substring (e.g., part of the filename or URL)
- Press g to toggle grouping by status
- Sorting works with or without grouping

## Speed and ETA

- Speed and ETA are shown for chunked and single-stream downloads
- Sampling is smoothed to avoid jitter on small files or low concurrency

## Handling authentication

- If a source requires authentication and tokens are missing, errors will indicate which env var is required (HF_TOKEN or CIVITAI_TOKEN)
- Consider adding tokens to your environment before launching the TUI if you plan to access gated content

## Troubleshooting

- No UI updates: ensure the terminal supports ANSI; try a different terminal emulator
- Open/Reveal fails: verify that `open` (macOS) or `xdg-open` (Linux) is installed and that the path exists
- Stuck resolving row: press X to clear the ephemeral; then retry with y
- Range/HEAD unsupported: downloader may fall back to single-stream; progress still updates with speed/ETA

## Configuration options (v2)

Add these under the ui section of your config YAML:

- ui.refresh_hz: integer (0-10). Controls refresh rate (ticks/sec). Default 1.
- ui.show_url: boolean. If true, the table shows URL instead of DEST by default. Deprecated by ui.column_mode when set.
- ui.column_mode: string: dest | url | host. Controls the last column.
- ui.compact: boolean. If true, uses compact table (STATUS, PROGRESS, ETA, URL/DEST/HOST).
- ui.theme: mapping of color overrides using 8-bit codes (e.g., border: "63").

## Recording sessions (asciinema)

- Install asciinema (macOS):

```bash
brew install asciinema
```

- Record a session (CTRL-D to stop):

```bash
asciinema rec -c "modfetch tui --config /path/to/config.yml --v2" out.cast
```

- Play locally:

```bash
asciinema play out.cast
```

- Upload/share:

```bash
asciinema upload out.cast
```

For animated SVGs/GIFs, consider tools like svg-term or agg (requires additional setup).

## Tips

- Quiet logs are for the CLI; the TUI always shows live state
- For large batches, consider using batch YAML with `modfetch download --batch jobs.yml` alongside the TUI
- Export MODFETCH_CONFIG for convenience so TUI and CLI share defaults

