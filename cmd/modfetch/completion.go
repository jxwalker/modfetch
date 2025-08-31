package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
)

func handleCompletion(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("completion", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("usage: modfetch completion [bash|zsh|fish]")
	}
	shell := fs.Arg(0)
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		return fmt.Errorf("unknown shell: %s", shell)
	}
	return nil
}

const bashCompletion = `# bash completion for modfetch
_modfetch_completions()
{
    local cur prev words cword
    _init_completion || return
    local cmds="config download place verify status tui batch version help completion"
    if [[ ${cword} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${cmds}" -- "$cur") )
        return
    fi
    case ${words[1]} in
        config)
            COMPREPLY=( $(compgen -W "validate print wizard --config --log-level --json" -- "$cur") ) ;;
        download)
            COMPREPLY=( $(compgen -W "--config --log-level --json --quiet --url --dest --sha256 --batch --place" -- "$cur") ) ;;
        place)
            COMPREPLY=( $(compgen -W "--config --log-level --json --path --type --mode" -- "$cur") ) ;;
        verify)
            COMPREPLY=( $(compgen -W "--config --path --all --log-level --json" -- "$cur") ) ;;
        status)
            COMPREPLY=( $(compgen -W "--config --log-level --json" -- "$cur") ) ;;
        tui)
            COMPREPLY=( $(compgen -W "--config --log-level --v1 --v2" -- "$cur") ) ;;
        batch)
            COMPREPLY=( $(compgen -W "import --config --log-level --json --input --output --dest-dir --sha-mode --type --place --mode --no-resolve-pages" -- "$cur") ) ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") ) ;;
        *) ;;
    esac
}
complete -F _modfetch_completions modfetch
`

const zshCompletion = `#compdef modfetch
# zsh completion for modfetch (basic)
_modfetch() {
  local -a cmds
  cmds=(config download place verify status tui batch version help completion)
  if (( CURRENT == 2 )); then
    _describe 'command' cmds
    return
  fi
  case $words[2] in
    config)
      _arguments '*:options:(--config --log-level --json validate print wizard)'
      ;;
    download)
      _arguments '*:options:(--config --log-level --json --quiet --url --dest --sha256 --batch --place)'
      ;;
    place)
      _arguments '*:options:(--config --log-level --json --path --type --mode)'
      ;;
    verify)
      _arguments '*:options:(--config --path --all --log-level --json)'
      ;;
    status)
      _arguments '*:options:(--config --log-level --json)'
      ;;
    tui)
      _arguments '*:options:(--config --log-level --v1 --v2)'
      ;;
    batch)
      _arguments '*:options:(import --config --log-level --json --input --output --dest-dir --sha-mode --type --place --mode --no-resolve-pages)'
      ;;
    completion)
      _arguments '*:options:(bash zsh fish)'
      ;;
  esac
}
compdef _modfetch modfetch
`

const fishCompletion = `# fish completion for modfetch
complete -c modfetch -f -n "__fish_use_subcommand" -a "config" -d "config ops"
complete -c modfetch -f -n "__fish_use_subcommand" -a "download" -d "download assets"
complete -c modfetch -f -n "__fish_use_subcommand" -a "place" -d "place files"
complete -c modfetch -f -n "__fish_use_subcommand" -a "verify" -d "verify checksums"
complete -c modfetch -f -n "__fish_use_subcommand" -a "status" -d "show status"
complete -c modfetch -f -n "__fish_use_subcommand" -a "tui" -d "dashboard"
complete -c modfetch -f -n "__fish_use_subcommand" -a "version" -d "print version"
complete -c modfetch -f -n "__fish_use_subcommand" -a "completion" -d "shell completions"

# Common flags
for cmd in config download place verify status tui batch
  complete -c modfetch -n "__fish_seen_subcommand_from $cmd" -l config -d "Path to config"
  complete -c modfetch -n "__fish_seen_subcommand_from $cmd" -l log-level -d "Log level"
end
complete -c modfetch -n "__fish_seen_subcommand_from download" -l url -d "URL or resolver URI"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l dest -d "Destination path"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l sha256 -d "Expected SHA256"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l batch -d "Batch file"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l place -d "Place after download"
# batch import flags
complete -c modfetch -n "__fish_seen_subcommand_from batch" -a "import" -d "Import URLs to YAML batch"
complete -c modfetch -n "__fish_seen_subcommand_from batch" -l input -d "Text file with URLs"
complete -c modfetch -n "__fish_seen_subcommand_from batch" -l output -d "Output batch YAML"
complete -c modfetch -n "__fish_seen_subcommand_from batch" -l dest-dir -d "Destination directory"
complete -c modfetch -n "__fish_seen_subcommand_from batch" -l sha-mode -d "none|compute"
complete -c modfetch -n "__fish_seen_subcommand_from batch" -l type -d "Artifact type"
complete -c modfetch -n "__fish_seen_subcommand_from batch" -l place -d "Place after download"
complete -c modfetch -n "__fish_seen_subcommand_from batch" -l mode -d "symlink|hardlink|copy"
complete -c modfetch -n "__fish_seen_subcommand_from batch" -l no-resolve-pages -d "Disable civitai page -> uri"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l path -d "File to place"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l type -d "Artifact type override"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l mode -d "Placement mode"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l path -d "File to verify"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l all -d "Verify all"
`
