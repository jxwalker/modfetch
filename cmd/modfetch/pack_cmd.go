package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jxwalker/modfetch/internal/taskpack"
)

func handlePack(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printPackUsage()
		if len(args) == 0 {
			return errors.New("pack subcommand required: list, show, export, download")
		}
		return nil
	}
	switch args[0] {
	case "list":
		return packList(args[1:])
	case "show":
		return packShow(args[1:])
	case "export":
		return packExport(args[1:])
	case "download":
		return packDownload(ctx, args[1:])
	default:
		return fmt.Errorf("unknown pack subcommand: %s", args[0])
	}
}

func printPackUsage() {
	fmt.Println(strings.TrimSpace(`Usage:
  modfetch pack list [--json]
  modfetch pack show ID [--json]
  modfetch pack export --id ID [--format batch|json] [--output PATH]
  modfetch pack download --id ID [--dry-run] [download flags]

Examples:
  modfetch pack list
  modfetch pack export --id llm-smoke --output llm-smoke.yml
  modfetch pack download --id embedding-smoke --dry-run
  modfetch pack download --id embedding-smoke --batch-parallel 2`))
}

func packList(args []string) error {
	fs := flag.NewFlagSet("pack list", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print pack catalog as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: modfetch pack list [--config PATH] [--log-level LEVEL] [--json]")
	}
	packs := taskpack.List()
	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(packs)
	}
	fmt.Println("Curated task packs:")
	for _, pack := range packs {
		fmt.Printf("  %-18s %-10s %2d files  %s\n", pack.ID, pack.Task, len(pack.Files), pack.Description)
	}
	fmt.Println("\nExport one with: modfetch pack export --id ID --output pack.yml")
	fmt.Println("Download one with: modfetch pack download --id ID")
	return nil
}

func packShow(args []string) error {
	fs := flag.NewFlagSet("pack show", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print pack details as JSON")
	flagArgs, idArgs := splitDiscoverArgs(args, map[string]bool{"json": true})
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	idArgs = append(idArgs, fs.Args()...)
	if len(idArgs) != 1 {
		return errors.New("usage: modfetch pack show [--config PATH] [--log-level LEVEL] [--json] ID")
	}
	pack, ok := taskpack.Get(idArgs[0])
	if !ok {
		return fmt.Errorf("unknown pack %q; run `modfetch pack list`", idArgs[0])
	}
	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pack)
	}
	fmt.Printf("%s (%s)\n", pack.Name, pack.ID)
	fmt.Printf("  Task:        %s\n", pack.Task)
	fmt.Printf("  Best for:    %s\n", pack.BestFor)
	fmt.Printf("  Description: %s\n", pack.Description)
	fmt.Println("  Files:")
	for _, file := range pack.Files {
		size := file.ApproxSize
		if strings.TrimSpace(size) == "" {
			size = "-"
		}
		fmt.Printf("    %-18s %-12s %s\n", file.Path, size, file.URI)
	}
	return nil
}

func packExport(args []string) error {
	fs := flag.NewFlagSet("pack export", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print pack manifest as JSON")
	id := fs.String("id", "", "pack ID from `modfetch pack list`")
	output := fs.String("output", "", "manifest output path (default: stdout)")
	format := fs.String("format", "batch", "manifest format: batch|json")
	destDir := fs.String("dest-dir", "", "destination root for generated batch jobs (default: config download_root)")
	flagArgs, idArgs := splitDiscoverArgs(args, map[string]bool{"json": true})
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	packID := strings.TrimSpace(*id)
	idArgs = append(idArgs, fs.Args()...)
	if packID == "" && len(idArgs) > 0 {
		packID = strings.TrimSpace(idArgs[0])
	}
	if packID == "" {
		return errors.New("pack export requires --id; run `modfetch pack list`")
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	root := strings.TrimSpace(*destDir)
	if root == "" {
		root = c.General.DownloadRoot
	}
	manifest, err := taskpack.MustManifest(packID, root)
	if err != nil {
		return err
	}
	if *common.jsonOut {
		*format = "json"
	}
	if err := writeManifestOutput(manifest, *format, *output); err != nil {
		return err
	}
	if strings.TrimSpace(*output) != "" {
		fmt.Fprintf(os.Stderr, "wrote %s manifest: %s (%d files)\n", normalizeManifestFormat(*format), *output, len(manifest.Files))
		if normalizeManifestFormat(*format) == "batch" {
			fmt.Fprintf(os.Stderr, "run with: modfetch download --batch %s [--batch-parallel N]\n", *output)
		}
	}
	return nil
}

func packDownload(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pack download", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print dry-run plan as JSON")
	id := fs.String("id", "", "pack ID from `modfetch pack list`")
	destDir := fs.String("dest-dir", "", "destination root for generated batch jobs (default: config download_root)")
	dryRun := fs.Bool("dry-run", false, "print the pack plan without downloading")
	batchParallel := fs.Int("batch-parallel", 0, "parallel file downloads")
	summaryJSON := fs.Bool("summary-json", false, "print per-file download summaries as JSON")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs")
	noResume := fs.Bool("no-resume", false, "start files fresh instead of resuming")
	profile := fs.String("profile", "auto", "download tuning profile: auto, default, or large-model")
	placeFlag := fs.Bool("place", false, "place files after successful download")
	mode := fs.String("mode", "", "placement mode: symlink|hardlink|copy")
	flagArgs, idArgs := splitDiscoverArgs(args, map[string]bool{
		"json": true, "dry-run": true, "summary-json": true, "quiet": true, "no-resume": true, "place": true,
	})
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	packID := strings.TrimSpace(*id)
	idArgs = append(idArgs, fs.Args()...)
	if packID == "" && len(idArgs) > 0 {
		packID = strings.TrimSpace(idArgs[0])
	}
	if packID == "" {
		return errors.New("pack download requires --id; run `modfetch pack list`")
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	root := strings.TrimSpace(*destDir)
	if root == "" {
		root = c.General.DownloadRoot
	}
	manifest, err := taskpack.MustManifest(packID, root)
	if err != nil {
		return err
	}
	if *dryRun {
		return printManifestPlan("Pack download plan (dry-run)", manifest, *common.jsonOut)
	}
	return runManifestDownload(ctx, manifest, manifestDownloadOptions{
		configPath:    *common.configPath,
		logLevel:      *common.logLevel,
		jsonOut:       *common.jsonOut,
		batchParallel: *batchParallel,
		summaryJSON:   *summaryJSON,
		quiet:         *quiet,
		noResume:      *noResume,
		profile:       *profile,
		place:         *placeFlag,
		mode:          *mode,
	})
}
