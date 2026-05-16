package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jxwalker/modfetch/internal/snapshot"
)

type repeatedStringFlag []string

func (f *repeatedStringFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func handleSnapshot(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printSnapshotUsage()
		if len(args) == 0 {
			return errors.New("usage: modfetch snapshot hf://owner/repo[/path] [flags]")
		}
		return nil
	}
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print snapshot manifest as JSON")
	var includes repeatedStringFlag
	var excludes repeatedStringFlag
	fs.Var(&includes, "include", "glob for files to include; repeatable (matches basename unless pattern contains /)")
	fs.Var(&excludes, "exclude", "glob for files to exclude; repeatable")
	rev := fs.String("rev", "", "repository revision (overrides rev query parameter)")
	output := fs.String("output", "", "manifest output path (default: stdout)")
	format := fs.String("format", "batch", "manifest format: batch|json")
	destDir := fs.String("dest-dir", "", "destination root for generated batch jobs (default: config download_root)")
	maxFiles := fs.Int("max-files", 100, "maximum files allowed in the manifest")
	download := fs.Bool("download", false, "download the generated manifest through the batch downloader")
	dryRun := fs.Bool("dry-run", false, "print the generated download plan without writing or downloading")
	batchParallel := fs.Int("batch-parallel", 0, "parallel file downloads when used with --download")
	summaryJSON := fs.Bool("summary-json", false, "print per-file download summaries as JSON with --download")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs with --download")
	noResume := fs.Bool("no-resume", false, "start files fresh instead of resuming with --download")
	profile := fs.String("profile", "auto", "download tuning profile with --download: auto, default, or large-model")
	placeFlag := fs.Bool("place", false, "place files after successful download")
	mode := fs.String("mode", "", "placement mode: symlink|hardlink|copy")
	flagArgs, uriArgs := splitDiscoverArgs(args, map[string]bool{
		"json": true, "download": true, "dry-run": true, "summary-json": true, "quiet": true, "no-resume": true, "place": true,
	})
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	uriArgs = append(uriArgs, fs.Args()...)
	if len(uriArgs) != 1 {
		return errors.New("usage: modfetch snapshot hf://owner/repo[/path] [flags]")
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	root := strings.TrimSpace(*destDir)
	if root == "" {
		root = c.General.DownloadRoot
	}
	manifest, err := snapshot.BuildHuggingFace(ctx, c, uriArgs[0], snapshot.Options{
		Rev:      *rev,
		Includes: includes,
		Excludes: excludes,
		DestDir:  root,
		MaxFiles: *maxFiles,
	})
	if err != nil {
		return err
	}
	if *dryRun {
		return printManifestPlan("Snapshot download plan (dry-run)", manifest, *common.jsonOut)
	}
	if *download {
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

func printSnapshotUsage() {
	fmt.Println(strings.TrimSpace(`Usage:
  modfetch snapshot hf://owner/repo[/path] [--include GLOB] [--output PATH]

Examples:
  modfetch snapshot hf://gpt2 --include 'tokenizer*' --include 'vocab.json'
  modfetch snapshot hf://hf-internal-testing/tiny-random-bert --include '*.json' --include '*.safetensors' --output tiny-bert.yml
  modfetch snapshot hf://owner/repo --include '*.gguf' --dry-run
  modfetch snapshot hf://owner/repo --include '*.gguf' --download --batch-parallel 2`))
}
