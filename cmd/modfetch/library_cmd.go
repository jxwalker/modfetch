package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jxwalker/modfetch/internal/catalog"
	"github.com/jxwalker/modfetch/internal/state"
)

func handleLibrary(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("library subcommand required: export or import")
	}
	switch args[0] {
	case "export":
		return handleLibraryExport(ctx, args[1:])
	case "import":
		return handleLibraryImport(ctx, args[1:])
	default:
		return fmt.Errorf("unknown library subcommand: %s", args[0])
	}
}

func handleLibraryExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("library export", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	format := fs.String("format", "json", "catalog format: json")
	output := fs.String("output", "-", "output path, or - for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if *format != "json" {
		return fmt.Errorf("unsupported export format %q", *format)
	}
	db, err := openLibraryDB(*common.configPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	w, closeFn, err := outputWriter(*output)
	if err != nil {
		return err
	}
	defer closeFn()
	return catalog.Export(db, w)
}

func handleLibraryImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("library import", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print import result as JSON")
	input := fs.String("input", "", "catalog JSON path, or - for stdin")
	dryRun := fs.Bool("dry-run", false, "report changes without writing to the library")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if *input == "" {
		return errors.New("library import requires --input")
	}
	db, err := openLibraryDB(*common.configPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	r, closeFn, err := inputReader(*input)
	if err != nil {
		return err
	}
	defer closeFn()
	result, err := catalog.Import(db, r, catalog.ImportOptions{DryRun: *dryRun})
	if err != nil {
		return err
	}
	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
		if result.Conflicts > 0 {
			return fmt.Errorf("catalog import has %d conflict(s)", result.Conflicts)
		}
		return nil
	}
	fmt.Printf("Import summary: creates=%d updates=%d skips=%d conflicts=%d dry_run=%t\n",
		result.Creates, result.Updates, result.Skips, result.Conflicts, result.DryRun)
	for _, entry := range result.Entries {
		if entry.Reason != "" {
			fmt.Printf("%s %s (%s)\n", entry.Action, entry.DownloadURL, entry.Reason)
		} else {
			fmt.Printf("%s %s\n", entry.Action, entry.DownloadURL)
		}
	}
	if result.Conflicts > 0 {
		return fmt.Errorf("catalog import has %d conflict(s)", result.Conflicts)
	}
	return nil
}

func openLibraryDB(configPath string) (*state.DB, error) {
	cfg, _, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}
	return state.Open(cfg)
}

func outputWriter(path string) (io.Writer, func(), error) {
	if path == "" || path == "-" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}

func inputReader(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}
