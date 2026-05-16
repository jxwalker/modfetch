package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jxwalker/modfetch/internal/batch"
	"github.com/jxwalker/modfetch/internal/snapshot"
)

type manifestDownloadOptions struct {
	configPath    string
	logLevel      string
	jsonOut       bool
	batchParallel int
	summaryJSON   bool
	quiet         bool
	noResume      bool
	profile       string
	place         bool
	mode          string
}

func writeManifestOutput(manifest *snapshot.Manifest, format, output string) error {
	format = normalizeManifestFormat(format)
	switch format {
	case "json":
		body, err := manifest.JSON()
		if err != nil {
			return err
		}
		body = append(body, '\n')
		if strings.TrimSpace(output) == "" {
			_, err = os.Stdout.Write(body)
			return err
		}
		return os.WriteFile(output, body, 0o644)
	case "batch":
		bf := manifest.Batch()
		if strings.TrimSpace(output) == "" {
			enc, _ := yamlEncoder()
			return enc.Encode(bf)
		}
		return batch.Save(output, bf)
	default:
		return fmt.Errorf("unknown manifest format %q (valid: batch, json)", format)
	}
}

func normalizeManifestFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "yaml", "yml", "batch":
		return "batch"
	case "json":
		return "json"
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func printManifestPlan(title string, manifest *snapshot.Manifest, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(manifest)
	}
	fmt.Printf("%s\n", title)
	fmt.Printf("  Source: %s\n", manifest.Source)
	fmt.Printf("  Repo:   %s\n", manifest.Repo)
	fmt.Printf("  Rev:    %s\n", manifest.Rev)
	if manifest.Prefix != "" {
		fmt.Printf("  Prefix: %s\n", manifest.Prefix)
	}
	fmt.Printf("  Files:  %d\n", len(manifest.Files))
	for i, file := range manifest.Files {
		fmt.Printf("  %2d. %s -> %s\n", i+1, file.URI, file.Dest)
	}
	return nil
}

func runManifestDownload(ctx context.Context, manifest *snapshot.Manifest, opts manifestDownloadOptions) error {
	tmp, err := os.CreateTemp("", "modfetch-manifest-*.yml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpPath) }()
	bf := manifest.Batch()
	for i := range bf.Jobs {
		if opts.place {
			bf.Jobs[i].Place = true
		}
		if strings.TrimSpace(opts.mode) != "" {
			bf.Jobs[i].Mode = strings.TrimSpace(opts.mode)
		}
	}
	if err := batch.Save(tmpPath, bf); err != nil {
		return err
	}
	args := []string{
		"--config", opts.configPath,
		"--log-level", opts.logLevel,
		"--batch", tmpPath,
	}
	if opts.jsonOut {
		args = append(args, "--json")
	}
	if opts.batchParallel > 0 {
		args = append(args, "--batch-parallel", fmt.Sprint(opts.batchParallel))
	}
	if opts.summaryJSON {
		args = append(args, "--summary-json")
	}
	if opts.quiet {
		args = append(args, "--quiet")
	}
	if opts.noResume {
		args = append(args, "--no-resume")
	}
	if strings.TrimSpace(opts.profile) != "" {
		args = append(args, "--profile", opts.profile)
	}
	return handleDownload(ctx, args)
}
