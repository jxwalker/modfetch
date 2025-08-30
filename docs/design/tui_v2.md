# TUI v2 RFC

Purpose
- Redesign the TUI to be modern, colorful, and clearly sectioned, with boxed panes, queues, and smooth realtime updates.

Goals
- Clear separation of queues: Pending | Active | Completed | Failed
- Boxed, themed layout with borders and badges; accessible high-contrast defaults
- Smooth progress with low jitter; per-row speed, ETA, optional sparkline
- Rich actions: start/pause/resume/cancel/retry, open/reveal, copy, delete staged
- Multi-select and batch actions
- Non-blocking toasts for completion/errors
- Configurable themes (Neon, Dracula, Solarized), border styles, table columns

Layout
- Header (boxed): totals, active throughput, error count, mini-sparklines
- Left (boxed): Queue tabs with counts
- Main (boxed): Items table for selected queue
- Right (boxed): Inspector with metadata and last log lines
- Footer (boxed): context-aware key hints

Keymap (initial)
- Global: 1..4 (queues), j/k (navigate), / (filter), g (group), s/e (sort speed/ETA), o (clear sort), t (toggle columns), T (theme)
- Row actions: Space (select), A (select all), p (pause/cancel), y (retry), O (open/reveal), C (copy path), U (copy URL), D (delete staged)
- Modal: n (new download), Enter (confirm), Esc (cancel)

Technical plan
- Keep Bubble Tea + Bubbles + Lipgloss
- Components: app, header, tabs, table, inspector, footer, toasts
- Theme system: central palette; unicode capability detection; ASCII fallback
- Event bus: single source of truth for row state; batched updates; throttled renders
- Config: YAML additions for theme, borders, refresh, columns
- Compatibility: macOS/Linux; Windows later
- Tests: state transitions, rendering snapshots per theme, perf guardrails

Milestones
- M1: Scaffold and theme system; static layout with fake data; --v2 flag
- M2: Live integration with downloader state; row actions; inspector
- M3: Filters/sorting/grouping; toasts; multi-select; perf tuning
- M4: Flip default; remove v1 after deprecation window; docs + screenshots

Risks
- Over-rendering; mitigate with harmonica and batched updates
- Terminal compatibility; provide ASCII fallback and color reductions


