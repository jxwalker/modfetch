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

<!-- TUI v2 is the default; no flag required -->

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
- Civitai model page URLs (https://civitai.com/models/ID) are accepted and auto-resolved to a direct file URL.

## Keybindings

The TUI now exposes key mappings via a discoverable help system. A concise
commands bar is always visible at the bottom, and pressing `?` toggles the full
help overlay.

Common keys include:

- `j`/`k` or arrow keys to navigate
- `n` to start a new download
- `b` to import a batch file
- `y` or `r` to start or retry
- `p` to cancel
- `D` to delete
- `O` to open the destination
- `/` to filter
- `s`/`e`/`R`/`o` to sort by speed, sort by ETA, sort by remaining bytes, or clear sorting
- `g` to group by host
- `t` to cycle the URL/DEST/HOST column
- `v` to toggle compact view
- `i` to toggle the inspector
- `H` to toggle the toast drawer
- `q` to quit

## Themes

- Presets: default, neon, drac, solar
- Press `T` to cycle through presets. The current theme name is shown in the Stats panel (right side), alongside Sort/Group/Column view indicators.
- The active sort is also reflected in the table header (SPEED*/ETA* or [sort: remaining]).

## Starting new downloads

Default naming
- The TUI derives a safe default destination inside your configured download_root.
- For civitai:// URIs, it uses the resolver’s SuggestedFilename (`<ModelName> - <OriginalFileName>`) with collision-safe suffixes when needed.
- For direct URLs, it uses the clean basename of the final URL (query/fragment stripped, sanitized).
- For Civitai direct download endpoints (`https://civitai.com/api/download/...`), the TUI tries a HEAD request to use the server-provided filename (`Content-Disposition`) when available; otherwise it falls back to the clean basename.

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

## Rate limiting

- If a source rate-limits your requests (HTTP 429), modfetch places the job on hold and surfaces this clearly:
  - Table shows status as `hold(rl)`.
  - The auth/status ribbon shows `rate-limited` for the affected host (Hugging Face or Civitai).
  - A toast appears with the host and may include a `Retry-After` hint from the server.
- You can retry with `y` or `r` later. Consider reducing concurrency, spacing out retries, or authenticating if the host enforces tighter limits for anonymous requests.
- Optional: set `network.retry_on_rate_limit: true` in your config to honor server-provided `Retry-After` between attempts.

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

