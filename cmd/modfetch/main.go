package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"modfetch/internal/config"
	"modfetch/internal/downloader"
	"modfetch/internal/logging"
	"modfetch/internal/resolver"
	"modfetch/internal/state"
	"modfetch/internal/placer"
	"modfetch/internal/batch"
	"modfetch/internal/metrics"
)

var version = "dev"

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
  config wizard     Interactive TUI to generate a YAML config
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
	case "wizard":
		return handleConfigWizard(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand: %s", sub)
	}
}

func handleDownload(args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs (errors only)")
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
	// quiet forces log level to error unless JSON is requested
	if *quiet && !*jsonOut { *logLevel = "error" }
	log := logging.New(*logLevel, *jsonOut)
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()
	ctx := context.Background()
	startWall := time.Now()
	// Metrics manager (Prometheus textfile), if enabled
	m := metrics.New(c)
	// Batch mode
	if *batchPath != "" {
		bf, err := batch.Load(*batchPath)
		if err != nil { return err }
		dl := downloader.NewChunked(c, log, st, m)
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
	} else {
		// Attach auth headers for direct URLs when possible
		if u, err := neturl.Parse(resolvedURL); err == nil {
			h := strings.ToLower(u.Hostname())
			if headers == nil { headers = map[string]string{} }
			if strings.HasSuffix(h, "civitai.com") && c.Sources.CivitAI.Enabled {
				if env := strings.TrimSpace(c.Sources.CivitAI.TokenEnv); env != "" {
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer " + tok }
				}
			}
			if strings.HasSuffix(h, "huggingface.co") && c.Sources.HuggingFace.Enabled {
				if env := strings.TrimSpace(c.Sources.HuggingFace.TokenEnv); env != "" {
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer " + tok }
				}
			}
		}
	}
	// Prefer chunked downloader; it will fall back to single when needed
	// Progress display (disabled for JSON or quiet)
	var stopProg func()
	if !*jsonOut && !*quiet {
		candDest := *dest
		if candDest == "" {
			candDest = filepath.Join(c.General.DownloadRoot, filepath.Base(resolvedURL))
		}
		stopProg = startProgressLoop(ctx, st, resolvedURL, candDest)
	}
	// Prefer chunked downloader; it will fall back to single when needed
	dl := downloader.NewChunked(c, log, st, m)
	final, sum, err := dl.Download(ctx, resolvedURL, *dest, *sha, headers)
	if stopProg != nil { stopProg() }
	if err != nil { return err }
	// Final summary
	if !*jsonOut {
		fi, _ := os.Stat(final)
		size := int64(0); if fi != nil { size = fi.Size() }
		dur := time.Since(startWall).Seconds()
		var rate string
		if dur > 0 && size > 0 { rate = humanize.Bytes(uint64(float64(size)/dur)) + "/s" } else { rate = "-" }
		fmt.Printf("\nDownloaded: %s\nDest: %s\nSHA256: %s\nSize: %s\nDuration: %.1fs  Avg: %s\n", resolvedURL, final, sum, humanize.Bytes(uint64(size)), dur, rate)
	}
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

