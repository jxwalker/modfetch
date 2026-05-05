package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/jxwalker/modfetch/internal/discovery"
)

func handleDiscover(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: modfetch discover [search|download] QUERY")
	}
	switch args[0] {
	case "search":
		return discoverSearch(ctx, args[1:])
	case "download":
		return discoverDownload(ctx, args[1:])
	case "help", "-h", "--help":
		printDiscoverUsage()
		return nil
	default:
		return discoverSearch(ctx, args)
	}
}

func printDiscoverUsage() {
	fmt.Println(strings.TrimSpace(`Usage:
  modfetch discover search QUERY [--provider huggingface|civitai|modelscope|all] [--limit N] [--json]
  modfetch discover download QUERY [--provider huggingface|civitai|modelscope|all] [--select N] [download flags]

Examples:
  modfetch discover search "tiny gpt2"
  modfetch discover download "sshleifer/tiny-gpt2" --select 1 --summary-json
  modfetch discover download "llama gguf" --select 2 --dry-run`))
}

func discoverSearch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("discover search", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print discovery results as JSON")
	provider := fs.String("provider", discovery.ProviderHuggingFace, "provider: huggingface|civitai|modelscope|all")
	limit := fs.Int("limit", 5, "maximum results to show")
	flagArgs, queryArgs := splitDiscoverArgs(args, map[string]bool{
		"json": true,
	})
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(append(queryArgs, fs.Args()...), " "))
	results, err := discovery.Search(ctx, discovery.Options{Provider: *provider, Query: query, Limit: *limit})
	if err != nil {
		return err
	}
	return printDiscoveryResults(results, query, *common.jsonOut)
}

func discoverDownload(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("discover download", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	provider := fs.String("provider", discovery.ProviderHuggingFace, "provider: huggingface|civitai|modelscope|all")
	limit := fs.Int("limit", 5, "maximum search results to inspect")
	selectIndex := fs.Int("select", 1, "1-based result index to download")
	dest := fs.String("dest", "", "destination path")
	placeFlag := fs.Bool("place", false, "place after successful download")
	summaryJSON := fs.Bool("summary-json", false, "print completion summary as JSON")
	dryRun := fs.Bool("dry-run", false, "plan without downloading")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs")
	noResume := fs.Bool("no-resume", false, "start fresh instead of resuming")
	flagArgs, queryArgs := splitDiscoverArgs(args, map[string]bool{
		"json": true, "place": true, "summary-json": true, "dry-run": true, "quiet": true, "no-resume": true,
	})
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(append(queryArgs, fs.Args()...), " "))
	results, err := discovery.Search(ctx, discovery.Options{Provider: *provider, Query: query, Limit: *limit})
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("no discovery results for %q", query)
	}
	if *selectIndex < 1 || *selectIndex > len(results) {
		return fmt.Errorf("--select must be between 1 and %d", len(results))
	}
	selected := results[*selectIndex-1]
	if !*quiet && !*common.jsonOut {
		fmt.Fprintf(os.Stderr, "Selected %d: %s (%s)\n", selected.Index, selected.Name, selected.URI)
	}
	downloadArgs := []string{
		"--config", *common.configPath,
		"--log-level", *common.logLevel,
		"--url", selected.URI,
	}
	if *common.jsonOut {
		downloadArgs = append(downloadArgs, "--json")
	}
	if strings.TrimSpace(*dest) != "" {
		downloadArgs = append(downloadArgs, "--dest", *dest)
	}
	if *placeFlag {
		downloadArgs = append(downloadArgs, "--place")
	}
	if *summaryJSON {
		downloadArgs = append(downloadArgs, "--summary-json")
	}
	if *dryRun {
		downloadArgs = append(downloadArgs, "--dry-run")
	}
	if *quiet {
		downloadArgs = append(downloadArgs, "--quiet")
	}
	if *noResume {
		downloadArgs = append(downloadArgs, "--no-resume")
	}
	return handleDownload(ctx, downloadArgs)
}

func printDiscoveryResults(results []discovery.Result, query string, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	if len(results) == 0 {
		fmt.Println("No matching models found.")
		return nil
	}
	fmt.Println("Real model candidates:")
	for _, result := range results {
		size := "-"
		if result.Size > 0 {
			size = humanize.Bytes(uint64(result.Size))
		}
		meta := strings.TrimSpace(result.Pipeline)
		if meta == "" {
			meta = strings.TrimSpace(result.Description)
		}
		if meta == "" {
			meta = result.Provider
		}
		fmt.Printf("%2d. %-36s %-12s downloads=%s likes=%s\n", result.Index, trimDisplay(result.Name, 36), size, humanize.Comma(result.Downloads), humanize.Comma(result.Likes))
		fmt.Printf("    %s\n", meta)
		if result.FilePath != "" || result.FileName != "" {
			file := result.FilePath
			if file == "" {
				file = result.FileName
			}
			fmt.Printf("    file: %s\n", file)
		}
		fmt.Printf("    uri:  %s\n", result.URI)
		fmt.Printf("    download: modfetch discover download %s --select %s\n", strconv.Quote(query), strconv.Itoa(result.Index))
	}
	return nil
}

func trimDisplay(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	if max <= 1 {
		return value[:max]
	}
	return value[:max-1] + "..."
}

func splitDiscoverArgs(args []string, boolFlags map[string]bool) ([]string, []string) {
	var flagArgs []string
	var queryArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			queryArgs = append(queryArgs, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			queryArgs = append(queryArgs, arg)
			continue
		}
		flagArgs = append(flagArgs, arg)
		name := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if strings.Contains(arg, "=") || boolFlags[name] {
			continue
		}
		if i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return flagArgs, queryArgs
}
