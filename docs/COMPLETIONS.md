# Shell completions

modfetch can generate shell completion scripts for bash, zsh, and fish.

Usage
- Bash:
  ```bash
  modfetch completion bash > /etc/bash_completion.d/modfetch
  # or for user:
  modfetch completion bash > ~/.local/share/bash-completion/modfetch
  . ~/.local/share/bash-completion/modfetch
  ```
- Zsh:
  ```bash
  # Ensure compinit is enabled in your ~/.zshrc
  autoload -U compinit && compinit
  modfetch completion zsh > ~/.zsh/completions/_modfetch
  fpath=(~/.zsh/completions $fpath)
  compinit
  ```
- Fish:
  ```bash
  modfetch completion fish > ~/.config/fish/completions/modfetch.fish
  ```

Notable flags and commands covered
- Commands: config, download, discover, get, recommend, pack, starter,
  snapshot, place, verify, status, tui, library, batch, dedupe, clean, hostcaps
- --url, --dest, --sha256, --place, --batch
- --quiet, --json, --log-level
- --summary-json, --no-resume, --batch-parallel, --naming-pattern, --no-auth-preflight, --dry-run
- --strict, --force, --quant, --list-quants
- library sync --target, --dry-run, --token-env
- pack --id, --format, --dest-dir, --batch-parallel
- snapshot --include, --exclude, --rev, --max-files, --download
- tui --snapshot for non-interactive state snapshots

Notes
- The provided completions are lightweight and cover subcommands and common flags. They do not shell out to the binary for dynamic completions.
- Regenerate completions after new releases to pick up added flags/commands,
  including nested library commands such as `library sync push/pull`.
