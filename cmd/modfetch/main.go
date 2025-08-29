package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"modfetch/internal/config"
	"modfetch/internal/downloader"
	"modfetch/internal/logging"
	"modfetch/internal/resolver"
	"modfetch/internal/state"
	"modfetch/internal/placer"
	"modfetch/internal/batch"
)

const version = "0.1.0-M0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no command provided")
	}

	// Global flags (parsed per subcommand to avoid hard defaults)
	cmd := args[0]
	switch cmd {
	case "config":
		return handleConfig(args[1:])
	case "status":
		return handleStatus(args[1:])
	case "download":
		return handleDownload(args[1:])
	case "place":
		return handlePlace(args[1:])
	case "verify":
		return handleVerify(args[1:])
	case "tui":
		return handleTUI(args[1:])
	case "version":
		fmt.Println(version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func usage() {
	fmt.Println(strings.TrimSpace(`modfetch - robust model fetcher (skeleton)

Usage:
  modfetch <command> [flags]

Commands:
  config validate   Validate a YAML config file
  config print      Print the loaded config as JSON
  download          Download a file via direct URL or resolver URI (hf://, civitai://)
  status            Show download status (table or JSON)
  place             Place a file into configured app directories
  verify            Verify SHA256 of a file or all completed downloads
  tui               Open the interactive terminal dashboard
  version           Print version
  help              Show this help

Flags:
  --config PATH     Path to YAML config file (or MODFETCH_CONFIG env var)
  --log-level L     Log level: debug|info|warn|error (per command)
  --json            JSON log output (per command)
`))
}

func handleStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	_ = c // currently unused; reserved for future filters
	log := logging.New(*logLevel, *jsonOut)
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()
	rows, err := st.ListDownloads()
	if err != nil { return err }
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	for _, r := range rows {
		log.Infof("%s -> %s [%s] size=%d", r.URL, r.Dest, r.Status, r.Size)
	}
	return nil
}

func handleConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("config subcommand required: validate | print")
	}
	sub := args[0]
	switch sub {
	case "validate":
		return configOp(args[1:], func(c *config.Config, log *logging.Logger) error {
			log.Infof("config: valid")
			return nil
		})
	case "print":
		return configOp(args[1:], func(c *config.Config, log *logging.Logger) error {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(c)
		})
	default:
		return fmt.Errorf("unknown config subcommand: %s", sub)
	}
}

func handleDownload(args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	url := fs.String("url", "", "HTTP URL to download (direct) or resolver URI (e.g. hf://owner/repo/path)")
	dest := fs.String("dest", "", "destination path (optional)")
	sha := fs.String("sha256", "", "expected SHA256 (optional)")
	batchPath := fs.String("batch", "", "YAML batch file with download jobs")
	placeFlag := fs.Bool("place", false, "place files after successful download")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env }
	}
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	log := logging.New(*logLevel, *jsonOut)
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()
	ctx := context.Background()
	// Batch mode
	if *batchPath != "" {
		bf, err := batch.Load(*batchPath)
		if err != nil { return err }
		dl := downloader.NewChunked(c, log, st)
		for i, job := range bf.Jobs {
			resolvedURL := job.URI
			headers := map[string]string{}
			if strings.HasPrefix(resolvedURL, "hf://") || strings.HasPrefix(resolvedURL, "civitai://") {
				res, err := resolver.Resolve(ctx, resolvedURL, c)
				if err != nil { return fmt.Errorf("job %d resolve: %w", i, err) }
				resolvedURL = res.URL
				headers = res.Headers
			}
			final, sum, err := dl.Download(ctx, resolvedURL, job.Dest, job.SHA256, headers)
			if err != nil { return fmt.Errorf("job %d download: %w", i, err) }
			log.Infof("downloaded: %s (sha256=%s)", final, sum)
			if job.Place || *placeFlag {
				placed, err := placer.Place(c, final, job.Type, job.Mode)
				if err != nil { return fmt.Errorf("job %d place: %w", i, err) }
				for _, p := range placed { log.Infof("placed: %s", p) }
			}
		}
		return nil
	}

	resolvedURL := *url
	headers := map[string]string{}
	if strings.HasPrefix(resolvedURL, "hf://") || strings.HasPrefix(resolvedURL, "civitai://") {
		res, err := resolver.Resolve(ctx, resolvedURL, c)
		if err != nil { return err }
		resolvedURL = res.URL
		headers = res.Headers
	}
	// Prefer chunked downloader; it will fall back to single when needed
	dl := downloader.NewChunked(c, log, st)
	final, sum, err := dl.Download(ctx, resolvedURL, *dest, *sha, headers)
	if err != nil { return err }
	log.Infof("downloaded: %s (sha256=%s)", final, sum)
	if *placeFlag {
		placed, err := placer.Place(c, final, "", "")
		if err != nil { return err }
		for _, p := range placed { log.Infof("placed: %s", p) }
	}
	return nil
}

func handlePlace(args []string) error {
	fs := flag.NewFlagSet("place", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	filePath := fs.String("path", "", "path to file to place")
	artType := fs.String("type", "", "artifact type override (optional)")
	mode := fs.String("mode", "", "placement mode override: symlink|hardlink|copy (optional)")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	if *filePath == "" { return errors.New("--path is required") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	log := logging.New(*logLevel, *jsonOut)
	placed, err := placer.Place(c, *filePath, *artType, *mode)
	if err != nil { return err }
	for _, p := range placed { log.Infof("placed: %s", p) }
	return nil
}

func configOp(args []string, fn func(*config.Config, *logging.Logger) error) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" {
			*cfgPath = env
		}
	}
	if *cfgPath == "" {
		return errors.New("--config is required or set MODFETCH_CONFIG")
	}
	c, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	log := logging.New(*logLevel, *jsonOut)
	return fn(c, log)
}

