package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jxwalker/modfetch/internal/starter"
)

func handleStarter(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return printStarterList(false)
	}
	switch args[0] {
	case "list", "ls":
		fs := flag.NewFlagSet("starter list", flag.ContinueOnError)
		jsonOut := fs.Bool("json", false, "print starter catalog as JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("usage: modfetch starter list [--json]")
		}
		return printStarterList(*jsonOut)
	case "show":
		fs := flag.NewFlagSet("starter show", flag.ContinueOnError)
		jsonOut := fs.Bool("json", false, "print starter entry as JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: modfetch starter show [--json] ID")
		}
		return printStarterEntry(fs.Arg(0), *jsonOut)
	case "download":
		return starterDownload(ctx, args[1:])
	case "help", "-h", "--help":
		printStarterUsage()
		return nil
	default:
		printStarterUsage()
		return fmt.Errorf("unknown starter subcommand: %s", args[0])
	}
}

func printStarterUsage() {
	fmt.Println(strings.TrimSpace(`Usage:
  modfetch starter list [--json]
  modfetch starter show [--json] ID
  modfetch starter download --id ID [--config PATH] [--dest PATH] [--dry-run] [--summary-json]

Examples:
  modfetch starter list
  modfetch starter download --id gpt2-config --summary-json
  modfetch download --url starter://gpt2-tokenizer`))
}

func printStarterList(jsonOut bool) error {
	entries := starter.List()
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	fmt.Println("Beginner-safe starter downloads:")
	for _, entry := range entries {
		fmt.Printf("  %-14s %-13s %-14s %8s  %s\n", entry.ID, entry.Provider, entry.Kind, entry.ApproxSize, entry.Name)
		fmt.Printf("    %s\n", entry.Description)
	}
	fmt.Println("\nDownload one with: modfetch starter download --id ID")
	return nil
}

func printStarterEntry(id string, jsonOut bool) error {
	entry, ok := starter.Get(id)
	if !ok {
		return fmt.Errorf("unknown starter %q", strings.TrimPrefix(strings.TrimSpace(id), "starter://"))
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entry)
	}
	fmt.Printf("%s\n", entry.Name)
	fmt.Printf("  ID:          %s\n", entry.ID)
	fmt.Printf("  Provider:    %s\n", entry.Provider)
	fmt.Printf("  Kind:        %s\n", entry.Kind)
	fmt.Printf("  URI:         starter://%s\n", entry.ID)
	fmt.Printf("  Underlying:  %s\n", entry.URI)
	fmt.Printf("  Approx size: %s\n", entry.ApproxSize)
	fmt.Printf("  Best for:    %s\n", entry.BestFor)
	fmt.Printf("  Notes:       %s\n", entry.Description)
	return nil
}

func starterDownload(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("starter download", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	id := fs.String("id", "", "starter ID from `modfetch starter list`")
	dest := fs.String("dest", "", "destination path (optional)")
	placeFlag := fs.Bool("place", false, "place after successful download")
	summaryJSON := fs.Bool("summary-json", false, "print JSON summary when complete")
	dryRun := fs.Bool("dry-run", false, "plan without downloading")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs")
	noResume := fs.Bool("no-resume", false, "do not resume; start fresh")
	if err := fs.Parse(args); err != nil {
		return err
	}
	starterID := strings.TrimSpace(*id)
	if starterID == "" && fs.NArg() > 0 {
		starterID = strings.TrimSpace(fs.Arg(0))
	}
	if _, ok := starter.Get(starterID); !ok {
		if starterID == "" {
			return errors.New("starter download requires --id; run `modfetch starter list`")
		}
		return fmt.Errorf("unknown starter %q; run `modfetch starter list`", starterID)
	}
	downloadArgs := []string{
		"--config", *common.configPath,
		"--log-level", *common.logLevel,
		"--url", "starter://" + strings.TrimPrefix(starterID, "starter://"),
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
