# Systemd user service for modfetch TUI

This guide shows how to run the modfetch TUI persistently via a systemd user service that wraps the TUI in a tmux session. This gives the TUI a TTY and allows attaching/detaching.

Important notes
- The TUI is interactive; systemd runs it in the background via tmux
- Do not put secrets (like tokens) in unit files. If you must define environment variables, prefer a user env file and restrict its permissions

Prerequisites
- tmux installed (e.g., `sudo apt-get install -y tmux`)
- A working modfetch binary on PATH for your user
- A valid config file (see docs/DEPLOY_LINUX.md and docs/CONFIG.md)

Environment file (optional but recommended)
Create a user environment file with your config path. Do not store secrets here.

- Path: `$HOME/.config/modfetch/modfetch.env`
- Example content:

```bash path=null start=null
# Path to your YAML config
MODFETCH_CONFIG=$HOME/.config/modfetch/config.yml
# Avoid putting secrets here; use your shell profile or a secrets manager
# HF_TOKEN=...
# CIVITAI_TOKEN=...
```

Install the user unit
1) Copy the provided unit file into your user systemd directory:

```bash path=null start=null
mkdir -p ~/.config/systemd/user
cp packaging/systemd/modfetch-tui.service ~/.config/systemd/user/
```

2) Reload and enable the service:

```bash path=null start=null
systemctl --user daemon-reload
systemctl --user enable --now modfetch-tui.service
```

3) Check logs:

```bash path=null start=null
journalctl --user -u modfetch-tui -f
```

Attach to the TUI
- The service runs the TUI in a tmux session named `modfetch_tui`.

```bash path=null start=null
tmux attach -t modfetch_tui
```

Stop the service

```bash path=null start=null
systemctl --user stop modfetch-tui.service
```

Troubleshooting
- `modfetch: command not found` in logs:
  - Ensure modfetch is on PATH for the user service; edit the unit to include your binary directory in `Environment=PATH=...`
- `tmux: not found`:
  - Install tmux (`sudo apt-get install -y tmux`) and reload the service
- Missing config:
  - Create `$HOME/.config/modfetch/modfetch.env` with `MODFETCH_CONFIG=...` or edit the unit to hardcode a `--config` path (not recommended)

