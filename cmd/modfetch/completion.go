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
    local presets="automatic1111 comfyui forge hf-cache ollama"
    local cmds="config download bench discover get recommend pack starter snapshot place verify status tui library batch dedupe clean hostcaps version help completion"
    if [[ ${cword} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${cmds}" -- "$cur") )
        return
    fi
	case ${words[1]} in
        config)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "validate print wizard" -- "$cur") )
                return
            fi
            case ${words[2]} in
                validate)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --strict" -- "$cur") ) ;;
                print)
                    COMPREPLY=( $(compgen -W "--config --log-level --json" -- "$cur") ) ;;
                wizard)
                    COMPREPLY=( $(compgen -W "--out" -- "$cur") ) ;;
                *) ;;
            esac ;;
        download)
            COMPREPLY=( $(compgen -W "--config --log-level --json --quiet --no-resume --url --dest --sha256 --sha256-file --batch --place --summary-json --batch-parallel --profile --connections --chunk-size-mb --dry-run --run-help --force --no-auth-preflight --extract --extract-dir --quant --list-quants" -- "$cur") ) ;;
        bench)
            COMPREPLY=( $(compgen -W "--config --log-level --json --url --tools --duration --profile --connections --chunk-size-mb --keep --history modfetch aria2" -- "$cur") ) ;;
        discover)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "search download" -- "$cur") )
                return
            fi
            case ${words[2]} in
                search)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --provider --limit huggingface civitai modelscope all" -- "$cur") ) ;;
                download)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --provider --limit --select --dest --place --summary-json --dry-run --run-help --quiet --no-resume huggingface civitai modelscope all" -- "$cur") ) ;;
                *) ;;
            esac ;;
        get)
            COMPREPLY=( $(compgen -W "--config --log-level --json --provider --query --limit --select --download --dest --place --summary-json --dry-run --run-help --quiet --no-resume --ram-gb --vram-gb --unified-memory --small --medium --large --size --starter-id --no-learn coding chat embedding embeddings image starter huggingface civitai modelscope all" -- "$cur") ) ;;
        recommend)
            COMPREPLY=( $(compgen -W "--config --log-level --json --provider --task --limit --ram-gb --vram-gb --unified-memory --select --download --dest --place --summary-json --dry-run --run-help --quiet --no-resume --history --history-limit --no-learn chat coding embedding image huggingface civitai modelscope all" -- "$cur") ) ;;
        pack)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "list show export download" -- "$cur") )
                return
            fi
            case ${words[2]} in
                list|show)
                    COMPREPLY=( $(compgen -W "--config --log-level --json llm-smoke embedding-smoke" -- "$cur") ) ;;
                export)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --id --output --format --dest-dir llm-smoke embedding-smoke batch json" -- "$cur") ) ;;
                download)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --id --dest-dir --dry-run --batch-parallel --summary-json --quiet --no-resume --profile --place --mode llm-smoke embedding-smoke" -- "$cur") ) ;;
                *) ;;
            esac ;;
        starter)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "list show download" -- "$cur") )
                return
            fi
            case ${words[2]} in
                list|show)
                    COMPREPLY=( $(compgen -W "--config --log-level --json gpt2-config gpt2-tokenizer public-1mb" -- "$cur") ) ;;
                download)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --id --dest --place --summary-json --dry-run --run-help --quiet --no-resume gpt2-config gpt2-tokenizer public-1mb" -- "$cur") ) ;;
                *) ;;
            esac ;;
        snapshot)
            COMPREPLY=( $(compgen -W "--config --log-level --json --include --exclude --rev --output --format --dest-dir --max-files --download --dry-run --batch-parallel --summary-json --quiet --no-resume --profile --place --mode batch json" -- "$cur") ) ;;
        dedupe)
            COMPREPLY=( $(compgen -W "--config --log-level --json --mode --dry-run" -- "$cur") ) ;;
        place)
            if [[ "$prev" == "--preset" ]]; then
                COMPREPLY=( $(compgen -W "${presets}" -- "$cur") )
                return
            fi
            COMPREPLY=( $(compgen -W "--config --log-level --json --path --type --mode --preset --list-presets --dry-run" -- "$cur") ) ;;
        verify)
            COMPREPLY=( $(compgen -W "--config --path --all --safetensors --safetensors-deep --scan-dir --repair --quarantine-incomplete --only-errors --summary --fix-sidecar --log-level --json" -- "$cur") ) ;;
        status)
            COMPREPLY=( $(compgen -W "--config --log-level --json --only-errors --summary --duplicates" -- "$cur") ) ;;
        tui)
            COMPREPLY=( $(compgen -W "--config --log-level --json --snapshot" -- "$cur") ) ;;
        library)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "export import scan sync" -- "$cur") )
                return
            fi
            case ${words[2]} in
                export)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --format --output" -- "$cur") ) ;;
                import)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --input --dry-run" -- "$cur") ) ;;
                scan)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --dir --workers --repair-stale --no-progress" -- "$cur") ) ;;
                sync)
                    if [[ ${cword} -eq 3 ]]; then
                        COMPREPLY=( $(compgen -W "push pull" -- "$cur") )
                    else
                        COMPREPLY=( $(compgen -W "--config --log-level --json --target --dry-run --token-env" -- "$cur") )
                    fi ;;
                *) ;;
            esac ;;
        clean)
            COMPREPLY=( $(compgen -W "--config --log-level --json --days --dry-run --dest --include-next-to-dest --sidecars" -- "$cur") ) ;;
        batch)
            if [[ ${cword} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "import" -- "$cur") )
                return
            fi
            case ${words[2]} in
                import)
                    COMPREPLY=( $(compgen -W "--config --log-level --json --input --output --dest-dir --sha-mode --type --place --mode --no-resolve-pages --naming-pattern" -- "$cur") ) ;;
                *) ;;
            esac ;;
        hostcaps)
            COMPREPLY=( $(compgen -W "--config --list --clear --clear-all --json" -- "$cur") ) ;;
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
  cmds=(config download bench discover get recommend pack starter snapshot place verify status tui library batch dedupe clean hostcaps version help completion)
  if (( CURRENT == 2 )); then
    _describe 'command' cmds
    return
  fi
  case $words[2] in
    config)
      if (( CURRENT == 3 )); then
        _arguments '*:subcommands:(validate print wizard)'
      else
        case $words[3] in
          validate)
            _arguments '*:options:(--config --log-level --json --strict)'
            ;;
          print)
            _arguments '*:options:(--config --log-level --json)'
            ;;
          wizard)
            _arguments '*:options:(--out)'
            ;;
        esac
      fi
      ;;
    download)
      _arguments '*:options:(--config --log-level --json --quiet --no-resume --url --dest --sha256 --sha256-file --batch --place --summary-json --batch-parallel --profile --connections --chunk-size-mb --dry-run --run-help --force --no-auth-preflight --extract --extract-dir --quant --list-quants)'
      ;;
    bench)
      _arguments '*:options:(--config --log-level --json --url --tools --duration --profile --connections --chunk-size-mb --keep --history modfetch aria2)'
      ;;
    discover)
      if (( CURRENT == 3 )); then
        _arguments '*:subcommands:(search download)'
      else
        case $words[3] in
          search)
            _arguments '*:options:(--config --log-level --json --provider --limit huggingface civitai modelscope all)'
            ;;
          download)
            _arguments '*:options:(--config --log-level --json --provider --limit --select --dest --place --summary-json --dry-run --run-help --quiet --no-resume huggingface civitai modelscope all)'
            ;;
        esac
      fi
      ;;
    get)
      _arguments '*:options:(--config --log-level --json --provider --query --limit --select --download --dest --place --summary-json --dry-run --run-help --quiet --no-resume --ram-gb --vram-gb --unified-memory --small --medium --large --size --starter-id --no-learn coding chat embedding embeddings image starter huggingface civitai modelscope all)'
      ;;
    recommend)
      _arguments '*:options:(--config --log-level --json --provider --task --limit --ram-gb --vram-gb --unified-memory --select --download --dest --place --summary-json --dry-run --run-help --quiet --no-resume --history --history-limit --no-learn chat coding embedding image huggingface civitai modelscope all)'
      ;;
    pack)
      if (( CURRENT == 3 )); then
        _arguments '*:subcommands:(list show export download)'
      else
        case $words[3] in
          list|show)
            _arguments '*:options:(--config --log-level --json llm-smoke embedding-smoke)'
            ;;
          export)
            _arguments '*:options:(--config --log-level --json --id --output --format --dest-dir llm-smoke embedding-smoke batch json)'
            ;;
          download)
            _arguments '*:options:(--config --log-level --json --id --dest-dir --dry-run --batch-parallel --summary-json --quiet --no-resume --profile --place --mode llm-smoke embedding-smoke)'
            ;;
        esac
      fi
      ;;
    starter)
      if (( CURRENT == 3 )); then
        _arguments '*:subcommands:(list show download)'
      else
        case $words[3] in
          list|show)
            _arguments '*:options:(--config --log-level --json gpt2-config gpt2-tokenizer public-1mb)'
            ;;
          download)
            _arguments '*:options:(--config --log-level --json --id --dest --place --summary-json --dry-run --run-help --quiet --no-resume gpt2-config gpt2-tokenizer public-1mb)'
            ;;
        esac
      fi
      ;;
    snapshot)
      _arguments '*:options:(--config --log-level --json --include --exclude --rev --output --format --dest-dir --max-files --download --dry-run --batch-parallel --summary-json --quiet --no-resume --profile --place --mode batch json)'
      ;;
    dedupe)
      _arguments '*:options:(--config --log-level --json --mode --dry-run)'
      ;;
    place)
      _arguments '--preset[Apply placement preset]:preset:_values -s , preset automatic1111 comfyui forge hf-cache ollama' '*:options:(--config --log-level --json --path --type --mode --list-presets --dry-run)'
      ;;
    verify)
      _arguments '*:options:(--config --path --all --safetensors --safetensors-deep --scan-dir --repair --quarantine-incomplete --only-errors --summary --fix-sidecar --log-level --json)'
      ;;
    status)
      _arguments '*:options:(--config --log-level --json --only-errors --summary --duplicates)'
      ;;
    tui)
      _arguments '*:options:(--config --log-level --json --snapshot)'
      ;;
    library)
      if (( CURRENT == 3 )); then
        _arguments '*:subcommands:(export import scan sync)'
      else
        case $words[3] in
          export)
            _arguments '*:options:(--config --log-level --json --format --output)'
            ;;
          import)
            _arguments '*:options:(--config --log-level --json --input --dry-run)'
            ;;
          scan)
            _arguments '*:options:(--config --log-level --json --dir --workers --repair-stale --no-progress)'
            ;;
          sync)
            if (( CURRENT == 4 )); then
              _arguments '*:subcommands:(push pull)'
            else
              _arguments '*:options:(--config --log-level --json --target --dry-run --token-env)'
            fi
            ;;
        esac
      fi
      ;;
    clean)
      _arguments '*:options:(--config --log-level --json --days --dry-run --dest --include-next-to-dest --sidecars)'
      ;;
    batch)
      if (( CURRENT == 3 )); then
        _arguments '*:subcommands:(import)'
      else
        case $words[3] in
          import)
            _arguments '*:options:(--config --log-level --json --input --output --dest-dir --sha-mode --type --place --mode --no-resolve-pages --naming-pattern)'
            ;;
        esac
      fi
      ;;
    hostcaps)
      _arguments '*:options:(--config --list --clear --clear-all --json)'
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
complete -c modfetch -f -n "__fish_use_subcommand" -a "bench" -d "benchmark download tools"
complete -c modfetch -f -n "__fish_use_subcommand" -a "discover" -d "search real model providers"
complete -c modfetch -f -n "__fish_use_subcommand" -a "get" -d "beginner task presets"
complete -c modfetch -f -n "__fish_use_subcommand" -a "recommend" -d "recommend models for hardware"
complete -c modfetch -f -n "__fish_use_subcommand" -a "pack" -d "curated task packs"
complete -c modfetch -f -n "__fish_use_subcommand" -a "starter" -d "beginner starter downloads"
complete -c modfetch -f -n "__fish_use_subcommand" -a "snapshot" -d "snapshot manifests"
complete -c modfetch -f -n "__fish_use_subcommand" -a "dedupe" -d "dedupe duplicate downloads"
complete -c modfetch -f -n "__fish_use_subcommand" -a "place" -d "place files"
complete -c modfetch -f -n "__fish_use_subcommand" -a "verify" -d "verify checksums"
complete -c modfetch -f -n "__fish_use_subcommand" -a "status" -d "show status"
complete -c modfetch -n "__fish_seen_subcommand_from status" -l only-errors -d "Only error rows"
complete -c modfetch -n "__fish_seen_subcommand_from status" -l summary -d "Print totals and errors"
complete -c modfetch -n "__fish_seen_subcommand_from status" -l duplicates -d "Show duplicate completed downloads"
complete -c modfetch -f -n "__fish_use_subcommand" -a "tui" -d "dashboard and snapshots"
complete -c modfetch -n "__fish_seen_subcommand_from tui" -l snapshot -d "Print state snapshot and exit"
complete -c modfetch -f -n "__fish_use_subcommand" -a "library" -d "library catalog"
complete -c modfetch -f -n "__fish_use_subcommand" -a "batch" -d "batch operations"
complete -c modfetch -f -n "__fish_use_subcommand" -a "version" -d "print version"
complete -c modfetch -f -n "__fish_use_subcommand" -a "help" -d "show help"
complete -c modfetch -f -n "__fish_use_subcommand" -a "completion" -d "shell completions"
complete -c modfetch -f -n "__fish_use_subcommand" -a "clean" -d "prune partials and sidecars"
complete -c modfetch -f -n "__fish_use_subcommand" -a "hostcaps" -d "host capability cache"
complete -c modfetch -n "__fish_seen_subcommand_from clean" -l days -d "Age threshold for .part"
complete -c modfetch -n "__fish_seen_subcommand_from clean" -l dry-run -d "Do not delete"
complete -c modfetch -n "__fish_seen_subcommand_from clean" -l dest -d "Target dest for staged .part"
complete -c modfetch -n "__fish_seen_subcommand_from clean" -l include-next-to-dest -d "Scan next-to-dest .part"
complete -c modfetch -n "__fish_seen_subcommand_from clean" -l sidecars -d "Remove orphan .sha256"

# Common flags
for cmd in download bench get recommend pack starter snapshot place verify status tui library dedupe clean
  complete -c modfetch -n "__fish_seen_subcommand_from $cmd" -l config -d "Path to config"
  complete -c modfetch -n "__fish_seen_subcommand_from $cmd" -l log-level -d "Log level"
  complete -c modfetch -n "__fish_seen_subcommand_from $cmd" -l json -d "JSON output"
end
complete -c modfetch -n "__fish_seen_subcommand_from config" -a "validate" -d "Validate config"
complete -c modfetch -n "__fish_seen_subcommand_from config" -a "print" -d "Print resolved config"
complete -c modfetch -n "__fish_seen_subcommand_from config" -a "wizard" -d "Create starter config"
complete -c modfetch -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from validate" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from validate" -l log-level -d "Log level"
complete -c modfetch -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from validate" -l json -d "JSON output"
complete -c modfetch -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from validate" -l strict -d "Reject unknown config fields"
complete -c modfetch -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from print" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from print" -l log-level -d "Log level"
complete -c modfetch -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from print" -l json -d "JSON output"
complete -c modfetch -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from wizard" -l out -d "Write wizard YAML to path"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l url -d "URL or resolver URI"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l dest -d "Destination path"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l sha256 -d "Expected SHA256"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l sha256-file -d "File containing expected hash"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l batch -d "Batch file"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l place -d "Place after download"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l quiet -d "Suppress progress and info logs"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l no-resume -d "Start fresh instead of resuming"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l summary-json -d "Print completion summary as JSON"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l batch-parallel -d "Parallel batch downloads"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l profile -d "Download tuning profile"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l connections -d "Parallel range requests per file"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l chunk-size-mb -d "Range chunk size in MiB"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l dry-run -d "Plan without downloading"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l run-help -d "Show local runtime guidance"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l force -d "Skip SHA256 verification"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l no-auth-preflight -d "Skip auth preflight probe"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l extract -d "Extract zip/tar/tar.gz/tgz/7z archive after download"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l extract-dir -d "Extraction directory"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l quant -d "HuggingFace quantization to download"
complete -c modfetch -n "__fish_seen_subcommand_from download" -l list-quants -d "List HuggingFace quantizations"
complete -c modfetch -n "__fish_seen_subcommand_from bench" -l url -d "URL or resolver URI to benchmark"
complete -c modfetch -n "__fish_seen_subcommand_from bench" -l tools -d "Tools to benchmark"
complete -c modfetch -n "__fish_seen_subcommand_from bench" -l duration -d "Sample duration per tool"
complete -c modfetch -n "__fish_seen_subcommand_from bench" -l profile -d "Download tuning profile"
complete -c modfetch -n "__fish_seen_subcommand_from bench" -l connections -d "Parallel range requests per file"
complete -c modfetch -n "__fish_seen_subcommand_from bench" -l chunk-size-mb -d "Range chunk size in MiB"
complete -c modfetch -n "__fish_seen_subcommand_from bench" -l keep -d "Keep benchmark downloads"
complete -c modfetch -n "__fish_seen_subcommand_from bench" -l history -d "List persisted benchmark history"
complete -c modfetch -n "__fish_seen_subcommand_from discover" -a "search" -d "Search real model providers"
complete -c modfetch -n "__fish_seen_subcommand_from discover" -a "download" -d "Download a selected discovery result"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from search" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from search" -l log-level -d "Log level"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from search" -l json -d "JSON output"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from search" -l provider -a "huggingface civitai modelscope all" -d "Provider"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from search" -l limit -d "Result limit"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l log-level -d "Log level"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l json -d "JSON output"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l provider -a "huggingface civitai modelscope all" -d "Provider"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l limit -d "Result limit"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l select -d "Result index"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l dest -d "Destination path"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l place -d "Place after download"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l summary-json -d "Print completion summary as JSON"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l dry-run -d "Plan without downloading"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l run-help -d "Show local runtime guidance"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l quiet -d "Suppress progress and info logs"
complete -c modfetch -n "__fish_seen_subcommand_from discover; and __fish_seen_subcommand_from download" -l no-resume -d "Start fresh instead of resuming"
complete -c modfetch -n "__fish_seen_subcommand_from get" -a "coding chat embedding embeddings image starter" -d "Task preset"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l provider -a "huggingface civitai modelscope all" -d "Provider"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l query -d "Override curated query"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l limit -d "Recommendation limit"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l select -d "Selected recommendation index"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l download -d "Download selected recommendation"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l dest -d "Destination path"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l place -d "Place after download"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l summary-json -d "Print completion summary as JSON"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l dry-run -d "Plan without downloading"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l run-help -d "Show local runtime guidance"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l quiet -d "Suppress progress and info logs"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l no-resume -d "Start fresh instead of resuming"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l ram-gb -d "Override RAM in GiB"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l vram-gb -d "Override VRAM in GiB"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l unified-memory -d "Treat RAM as unified memory"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l small -d "Prefer a small first download"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l medium -d "Prefer a balanced local model"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l large -d "Prefer larger candidates"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l size -a "small medium large" -d "Size preset"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l starter-id -a "gpt2-config gpt2-tokenizer public-1mb" -d "Starter ID"
complete -c modfetch -n "__fish_seen_subcommand_from get" -l no-learn -d "Disable recommendation history"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l provider -a "huggingface civitai modelscope all" -d "Provider"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l task -a "chat coding embedding image" -d "Use case"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l limit -d "Recommendation limit"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l ram-gb -d "Override RAM in GiB"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l vram-gb -d "Override VRAM in GiB"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l unified-memory -d "Treat RAM as unified memory"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l select -d "Selected recommendation index"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l download -d "Download selected recommendation"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l dest -d "Destination path"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l place -d "Place after download"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l summary-json -d "Print completion summary as JSON"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l dry-run -d "Plan without downloading"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l run-help -d "Show local runtime guidance"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l quiet -d "Suppress progress and info logs"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l no-resume -d "Start fresh instead of resuming"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l history -d "List recommendation history"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l history-limit -d "Recommendation history row limit"
complete -c modfetch -n "__fish_seen_subcommand_from recommend" -l no-learn -d "Disable recommendation history for this invocation"
complete -c modfetch -n "__fish_seen_subcommand_from pack" -a "list" -d "List curated packs"
complete -c modfetch -n "__fish_seen_subcommand_from pack" -a "show" -d "Show pack details"
complete -c modfetch -n "__fish_seen_subcommand_from pack" -a "export" -d "Export a pack manifest"
complete -c modfetch -n "__fish_seen_subcommand_from pack" -a "download" -d "Download a pack"
complete -c modfetch -n "__fish_seen_subcommand_from pack" -a "llm-smoke embedding-smoke" -d "Pack ID"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from export" -l id -a "llm-smoke embedding-smoke" -d "Pack ID"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from export" -l output -d "Manifest output path"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from export" -l format -a "batch json" -d "Manifest format"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from export" -l dest-dir -d "Destination root"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l id -a "llm-smoke embedding-smoke" -d "Pack ID"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l dest-dir -d "Destination root"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l dry-run -d "Plan without downloading"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l batch-parallel -d "Parallel file downloads"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l summary-json -d "Print per-file summaries"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l quiet -d "Suppress progress and info logs"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l no-resume -d "Start fresh instead of resuming"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l profile -a "auto default large-model" -d "Download tuning profile"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l place -d "Place files after download"
complete -c modfetch -n "__fish_seen_subcommand_from pack; and __fish_seen_subcommand_from download" -l mode -a "symlink hardlink copy" -d "Placement mode"
complete -c modfetch -n "__fish_seen_subcommand_from starter" -a "list" -d "List starter downloads"
complete -c modfetch -n "__fish_seen_subcommand_from starter" -a "show" -d "Show starter details"
complete -c modfetch -n "__fish_seen_subcommand_from starter" -a "download" -d "Download a starter"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from list" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from list" -l log-level -d "Log level"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from list" -l json -d "JSON output"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from show" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from show" -l log-level -d "Log level"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from show" -l json -d "JSON output"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from show" -a "gpt2-config gpt2-tokenizer public-1mb" -d "Starter ID"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l log-level -d "Log level"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l json -d "JSON output"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l id -a "gpt2-config gpt2-tokenizer public-1mb" -d "Starter ID"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l dest -d "Destination path"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l place -d "Place after download"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l summary-json -d "Print completion summary as JSON"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l dry-run -d "Plan without downloading"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l run-help -d "Show local runtime guidance"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l quiet -d "Suppress progress and info logs"
complete -c modfetch -n "__fish_seen_subcommand_from starter; and __fish_seen_subcommand_from download" -l no-resume -d "Start fresh instead of resuming"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l include -d "Include glob"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l exclude -d "Exclude glob"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l rev -d "Repository revision"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l output -d "Manifest output path"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l format -a "batch json" -d "Manifest format"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l dest-dir -d "Destination root"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l max-files -d "Maximum matched files"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l download -d "Download generated manifest"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l dry-run -d "Plan without downloading"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l batch-parallel -d "Parallel file downloads"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l summary-json -d "Print per-file summaries"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l quiet -d "Suppress progress and info logs"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l no-resume -d "Start fresh instead of resuming"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l profile -a "auto default large-model" -d "Download tuning profile"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l place -d "Place files after download"
complete -c modfetch -n "__fish_seen_subcommand_from snapshot" -l mode -a "symlink hardlink copy" -d "Placement mode"
complete -c modfetch -n "__fish_seen_subcommand_from library" -a "export" -d "Export model catalog"
complete -c modfetch -n "__fish_seen_subcommand_from library" -a "import" -d "Import model catalog"
complete -c modfetch -n "__fish_seen_subcommand_from library" -a "scan" -d "Scan model directories"
complete -c modfetch -n "__fish_seen_subcommand_from library" -a "sync" -d "Sync model catalog"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from export" -l format -d "Catalog format"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from export" -l output -d "Output catalog path"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from import" -l input -d "Input catalog path"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from import" -l dry-run -d "Report changes without writing"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from scan" -l dir -d "Directory to scan"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from scan" -l workers -d "Scanner worker count"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from scan" -l repair-stale -d "Remove metadata for missing files"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from scan" -l no-progress -d "Disable progress output"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from sync" -a "push" -d "Push catalog to target"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from sync" -a "pull" -d "Pull catalog from target"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from sync" -l target -d "Catalog sync target"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from sync" -l dry-run -d "Report without writing"
complete -c modfetch -n "__fish_seen_subcommand_from library; and __fish_seen_subcommand_from sync" -l token-env -d "Bearer token environment variable"
complete -c modfetch -n "__fish_seen_subcommand_from dedupe" -l mode -d "hardlink|symlink"
complete -c modfetch -n "__fish_seen_subcommand_from dedupe" -l dry-run -d "Show dedupe changes without modifying files"
# batch import flags
complete -c modfetch -n "__fish_seen_subcommand_from batch" -a "import" -d "Import URLs to YAML batch"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l log-level -d "Log level"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l json -d "JSON output"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l input -d "Text file with URLs"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l output -d "Output batch YAML"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l dest-dir -d "Destination directory"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l sha-mode -d "none|compute"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l type -d "Artifact type"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l place -d "Place after download"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l mode -d "symlink|hardlink|copy"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l no-resolve-pages -d "Disable civitai page -> uri"
complete -c modfetch -n "__fish_seen_subcommand_from batch; and __fish_seen_subcommand_from import" -l naming-pattern -d "Override resolver naming pattern"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l path -d "File to place"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l type -d "Artifact type override"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l mode -d "Placement mode"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l preset -a "automatic1111 comfyui forge hf-cache ollama" -d "Apply placement preset"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l list-presets -d "List placement presets"
complete -c modfetch -n "__fish_seen_subcommand_from place" -l dry-run -d "Show planned placements only"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l path -d "File to verify"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l all -d "Verify all"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l safetensors -d "Check .safetensors structure"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l safetensors-deep -d "Deep-verify .safetensors"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l scan-dir -d "Scan directory for .safetensors"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l repair -d "Trim extra bytes on deep verify"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l quarantine-incomplete -d "Quarantine incomplete files"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l only-errors -d "Show only errors"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l summary -d "Print summary"
complete -c modfetch -n "__fish_seen_subcommand_from verify" -l fix-sidecar -d "Rewrite .sha256 sidecar on verified"
complete -c modfetch -n "__fish_seen_subcommand_from hostcaps" -l config -d "Path to config"
complete -c modfetch -n "__fish_seen_subcommand_from hostcaps" -l list -d "List cached host capabilities"
complete -c modfetch -n "__fish_seen_subcommand_from hostcaps" -l clear -d "Clear cache for a host"
complete -c modfetch -n "__fish_seen_subcommand_from hostcaps" -l clear-all -d "Clear all cached host capabilities"
complete -c modfetch -n "__fish_seen_subcommand_from hostcaps" -l json -d "JSON output"
`
